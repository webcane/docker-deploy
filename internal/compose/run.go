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

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"

	"github.com/mniedre/docker-deploy/internal/filetransfer"
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
// A fresh NewSession() is opened per CLAUDE.md Rule 3; the session is closed
// after Wait() returns.
//
// remotePath is wrapped in filetransfer.ShellQuote() to prevent shell
// injection from paths containing spaces or special characters (T-04-02-01).
// composeFile must be a validated basename (no slashes) — callers are
// responsible for filepath.Base() validation before calling RunCompose
// (T-04-02-02, see Plan 01 SUMMARY threat flags).
func RunCompose(ctx context.Context, client *gossh.Client, remotePath, composeFile string) error {
	// Construct the remote command. remotePath is shell-quoted; composeFile is a
	// validated basename and does not need quoting.
	cmd := "docker compose -f " + filetransfer.ShellQuote(remotePath) + "/" + composeFile + " up -d --remove-orphans"

	// Open a dedicated session per CLAUDE.md Rule 3 (sessions are NOT reusable).
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("opening compose session: %w", err)
	}
	defer session.Close() //nolint:errcheck

	// TTY detection: decide between PTY allocation and goroutine pipe drains.
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	if isTTY {
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
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			io.Copy(os.Stdout, stdoutPipe) //nolint:errcheck
		}()
		go func() {
			defer wg.Done()
			io.Copy(os.Stderr, stderrPipe) //nolint:errcheck
		}()
		// Start the command; drain goroutines; then wait for exit.
		if startErr := session.Start(cmd); startErr != nil {
			return fmt.Errorf("starting compose session: %w", startErr)
		}
		// Wait for both drains to complete before calling session.Wait() to
		// ensure all output is flushed (prevents truncated log lines in CI).
		wg.Wait()
		return handleWait(session.Wait())
	}

	// PTY path: start and wait inline (PTY drains synchronously).
	if startErr := session.Start(cmd); startErr != nil {
		return fmt.Errorf("starting compose session: %w", startErr)
	}
	return handleWait(session.Wait())
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
