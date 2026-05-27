// Package compose provides the RunCompose function which executes docker
// compose up on a remote host via SSH and streams its output locally.
package compose

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"unicode"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"github.com/webcane/docker-deploy/internal/filetransfer"
)

// RunCompose executes "docker compose -f <remotePath>/<composeFile> up -d
// --remove-orphans" on the remote host connected via client.
//
// Output routing (per CLAUDE.md and D-01–D-03 in CONTEXT.md):
//   - When local stdout is a real TTY: allocates a PTY (xterm-256color) so
//     compose colour output renders correctly. Merged stdout+stderr go to
//     os.Stdout via the PTY.
//   - When local stdout is not a TTY (CI, piped output): two goroutines
//     drain session.StdoutPipe→os.Stdout and session.StderrPipe→os.Stderr.
//
// Exit code propagation (per D-12, DEPLOY-05):
//   - Exit 0 → nil error.
//   - Exit N → writes "Deploy failed: docker compose exited with code N" to
//     os.Stderr and returns a non-nil error with the same message.
//
// Verbose logging (per D-01, Phase 7 Plan 02):
//   - When verbose=true: logs "[ssh] <cmd>" to os.Stderr before session.Start(),
//     and "  → exit 0" or "  → exit N" after Wait() returns.
//   - When verbose=false: no additional output.
//
// A fresh NewSession() is opened per CLAUDE.md Rule 3; the session is closed
// after Wait() returns.
//
// The combined path (remotePath + "/" + composeFile) is wrapped in
// filetransfer.ShellQuote() as a single token to prevent shell injection from
// either path component (T-04-02-01, CR-01). composeFile is additionally
// validated against an alphanumeric allowlist (letters, digits, '-', '_', '.')
// before quoting (T-04-02-02) — filepath.Base() at the call site prevents path
// separators, but does not strip shell-active characters like ';', '|', '$',
// or '`'.
func RunCompose(ctx context.Context, client *gossh.Client, remotePath, composeFile string, verbose bool) error { //nolint:gocognit // complexity comes from TTY vs non-TTY branching and signal handling — splitting would require passing shared state across helper funcs
	// Allowlist validation: reject any character that is not alphanumeric, '-',
	// '_', or '.'. This guards against injection even before ShellQuote is
	// applied, providing defence-in-depth (T-04-02-02).
	if !isValidComposeFilename(composeFile) {
		return fmt.Errorf("compose file contains invalid characters: %q (only letters, digits, '-', '_', '.' are allowed)", composeFile)
	}

	// Construct the remote command. The combined path (remotePath + "/" + composeFile)
	// is shell-quoted as a single token so that neither remotePath nor composeFile
	// can inject shell metacharacters (CR-01, T-04-02-01).
	combinedPath := remotePath + "/" + composeFile
	cmd := "docker compose -f " + filetransfer.ShellQuote(combinedPath) + " up -d --remove-orphans"

	// Open a dedicated session per CLAUDE.md Rule 3 (sessions are NOT reusable).
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("opening compose session: %w", err)
	}
	// Derive a child context so the cancellation watcher goroutine below can be
	// stopped cleanly when RunCompose returns normally (WR-02).
	ctx, cancel := context.WithCancel(ctx)
	// Watch for context cancellation and close the SSH session so that
	// session.Wait() and io.Copy unblock promptly (e.g. on Ctrl-C).
	// Use WaitGroup to ensure the goroutine exits before RunCompose returns (WR-02).
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		session.Close() //nolint:errcheck,gosec // G104: intentionally unhandled — error is informational or cleanup
	}()
	// Defer cleanup in the correct order: cancel context first, then wait for goroutine.
	// This prevents deadlock where wg.Wait() blocks before ctx.Done() is signaled.
	defer func() {
		cancel()
		wg.Wait()
	}()
	// The deferred close below handles the normal (non-cancelled) exit path.
	// When the context is cancelled the goroutine above closes the session first;
	// the subsequent defer call is a no-op (double-close is safe for gossh).
	// io.EOF and syscall.EPIPE are expected errors when closing after the remote
	// process has exited or when the connection is broken; suppress them to avoid
	// noisy output in concurrent deployments (WR-04).
	defer func() {
		if closeErr := session.Close(); closeErr != nil {
			if !errors.Is(closeErr, io.EOF) && !errors.Is(closeErr, syscall.EPIPE) {
				fmt.Fprintf(os.Stderr, "warning: session close: %v\n", closeErr)
			}
		}
	}()

	// TTY detection: decide between PTY allocation and goroutine pipe drains.
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	if isTTY { //nolint:nestif // TTY and non-TTY paths share session lifecycle — extracting would require significant shared-state plumbing
		// PTY path: allocate a pseudo-terminal so compose renders colours.
		w, h, sizeErr := term.GetSize(int(os.Stdout.Fd()))
		if sizeErr != nil {
			// Sensible fallback dimensions if the terminal size cannot be
			// determined (e.g. Stdout is a PTY not attached to a real window).
			w, h = 80, 24
		}
		modes := gossh.TerminalModes{
			gossh.ECHO:          0,
			gossh.TTY_OP_ISPEED: 14400,
			gossh.TTY_OP_OSPEED: 14400,
		}
		if ptErr := session.RequestPty("xterm-256color", h, w, modes); ptErr != nil {
			return fmt.Errorf("requesting PTY: %w", ptErr)
		}
		// PTY merges stdout+stderr — both go to os.Stdout per D-01.
		// session.Stdin is intentionally NOT connected to os.Stdin to prevent
		// interactive input from reaching the remote shell (T-04-02-03).
		session.Stdout = os.Stdout
		session.Stderr = os.Stdout // intentional: PTY merges streams
	} else {
		// Non-TTY path: drain stdout and stderr via two goroutines so neither
		// pipe blocks compose from writing (T-04-02-04: sync.WaitGroup prevents
		// goroutine leak).
		stdoutPipe, pipeErr := session.StdoutPipe()
		if pipeErr != nil {
			return fmt.Errorf("getting stdout pipe: %w", pipeErr)
		}
		stderrPipe, pipeErr := session.StderrPipe()
		if pipeErr != nil {
			return fmt.Errorf("getting stderr pipe: %w", pipeErr)
		}
		// Start the command BEFORE launching goroutines. If Start fails, no
		// goroutines have been spawned, so there is nothing to leak (CR-02).
		if verbose {
			fmt.Fprintf(os.Stderr, "[ssh] %s\n", cmd)
		}
		if startErr := session.Start(cmd); startErr != nil {
			return fmt.Errorf("starting compose session: %w", startErr)
		}
		// Pipes are live — launch drain goroutines now that Start succeeded.
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			io.Copy(os.Stdout, stdoutPipe) //nolint:errcheck,gosec // G104: intentionally unhandled — error is informational or cleanup
		}()
		go func() {
			defer wg.Done()
			io.Copy(os.Stderr, stderrPipe) //nolint:errcheck,gosec // G104: intentionally unhandled — error is informational or cleanup
		}()
		// Wait for both drains to complete before calling session.Wait() to
		// ensure all output is flushed (prevents truncated log lines in CI).
		wg.Wait()
		waitErr := handleWait(session.Wait())
		if verbose && waitErr == nil {
			fmt.Fprintf(os.Stderr, "  → exit 0\n")
		} else if verbose && waitErr != nil {
			var exitErr *gossh.ExitError
			if errors.As(waitErr, &exitErr) {
				fmt.Fprintf(os.Stderr, "  → exit %d\n", exitErr.ExitStatus())
			}
		}
		return waitErr
	}

	// PTY path: start and wait inline (PTY drains synchronously).
	if verbose {
		fmt.Fprintf(os.Stderr, "[ssh] %s\n", cmd)
	}
	if startErr := session.Start(cmd); startErr != nil {
		return fmt.Errorf("starting compose session: %w", startErr)
	}
	waitErr := handleWait(session.Wait())
	if verbose && waitErr == nil {
		fmt.Fprintf(os.Stderr, "  → exit 0\n")
	} else if verbose && waitErr != nil {
		var exitErr *gossh.ExitError
		if errors.As(waitErr, &exitErr) {
			fmt.Fprintf(os.Stderr, "  → exit %d\n", exitErr.ExitStatus())
		}
	}
	return waitErr
}

// isValidComposeFilename returns true if s contains only letters, digits, '-',
// '_', or '.' and is non-empty. This provides an allowlist guard against shell
// injection via the compose-file value (T-04-02-02).
func isValidComposeFilename(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
}

// handleWait converts session.Wait() errors into structured RunCompose errors.
//
// T-04-02-05: errors.As(waitErr, &exitErr) is the idiomatic gossh pattern.
// Non-ExitError errors are wrapped and returned, not silently swallowed.
func handleWait(waitErr error) error {
	if waitErr == nil {
		return nil
	}
	var exitErr *gossh.ExitError
	if errors.As(waitErr, &exitErr) {
		code := exitErr.ExitStatus()
		fmt.Fprintf(os.Stderr, "Deploy failed: docker compose exited with code %d\n", code)
		return fmt.Errorf("docker compose exited with code %d", code)
	}
	return fmt.Errorf("compose session wait: %w", waitErr)
}
