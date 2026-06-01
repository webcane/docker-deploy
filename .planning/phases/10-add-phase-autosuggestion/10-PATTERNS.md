# Phase 10: Add Phase Autosuggestion - Pattern Map

**Mapped:** 2026-06-01
**Files analyzed:** 4 (2 modified, 2 test additions)
**Analogs found:** 4 / 4

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/docker-deploy/main.go` | controller | request-response | self (existing `buildVersionCmd`, `buildValidateCmd`) | exact |
| `internal/sshconfig/sshconfig.go` | utility | transform | self (existing `LookupHost` scanner pattern) | exact |
| `internal/sshconfig/sshconfig_test.go` | test | — | self (existing `TestLookupHost_*` pattern) | exact |
| `cmd/docker-deploy/main_test.go` | test | — | self (existing `TestVersionCmd_Registered` pattern) | exact |

## Pattern Assignments

---

### `cmd/docker-deploy/main.go` — add `buildCompletionCmd()` + `RegisterFlagCompletionFunc` calls

**Analog:** `buildVersionCmd()` and `buildValidateCmd()` in the same file.

**Imports pattern** (lines 5–28 — no new imports needed for the completion subcommand itself; add `sshconfig` import alias and ensure `os`, `filepath` are present):
```go
import (
    "os"
    "path/filepath"
    // already present:
    "github.com/spf13/cobra"
    "github.com/webcane/docker-deploy/internal/config"
    sshpkg "github.com/webcane/docker-deploy/internal/ssh"
    // add:
    "github.com/webcane/docker-deploy/internal/sshconfig"
)
```
Note: `os` and `filepath` are already imported. `sshconfig` is NOT yet imported in `main.go` — it must be added. The package is at `github.com/webcane/docker-deploy/internal/sshconfig`.

**Subcommand factory pattern** (analog: `buildVersionCmd` lines 99–108, `buildValidateCmd` lines 136–145):
```go
// buildVersionCmd — the exact factory shape to copy for buildCompletionCmd:
func buildVersionCmd() *cobra.Command {
    return &cobra.Command{
        Use:          "version",
        Short:        "Print version information",
        SilenceUsage: true,
        RunE: func(_ *cobra.Command, _ []string) error {
            return runVersion()
        },
    }
}
```
`buildCompletionCmd` follows this shape but adds `ValidArgs` and `Args` because the shell name is a positional argument.

**Subcommand registration pattern** (analog: lines 91–94 in `buildDeployCmd`):
```go
cmd.AddCommand(buildVersionCmd())
cmd.AddCommand(buildValidateCmd())
// New registration goes here, before return cmd:
cmd.AddCommand(buildCompletionCmd())
```

**Flag completion registration placement** — after all `cmd.Flags().*Var()` calls and before `cmd.AddCommand()` calls (lines 79–94). The three `RegisterFlagCompletionFunc` calls insert between line 90 (`cmd.Flags().BoolVar(&verbose, ...)`) and line 92 (`cmd.AddCommand(buildVersionCmd())`).

**`os.Getwd()` + `filepath.Base()` pattern** (analog: `runValidate` lines 152–154 and `runDeploy` lines 340–344):
```go
cwd, err := os.Getwd()
if err != nil {
    return fmt.Errorf("getting working directory: %w", err)
}
projectName := filepath.Base(cwd)
```
In a completion function this becomes a silent-failure variant (no error return):
```go
cwd, err := os.Getwd()
if err != nil {
    return nil, cobra.ShellCompDirectiveNoFileComp
}
suggestion := "/opt/" + filepath.Base(cwd)
```

**`config.LoadFile()` call pattern** (analog: `runValidate` lines 171–174 and `runDeploy` lines 347–350):
```go
fileConfig, _, err := config.LoadFile(cwd)
if err != nil {
    return fmt.Errorf("loading deploy.yaml: %w", err)
}
```
In a completion function this becomes silent on error:
```go
if fc, _, err := config.LoadFile(cwd); err == nil && fc.Target.Host != "" {
    hosts = append(hosts, fc.Target.Host)
}
```

**`os.UserHomeDir()` pattern** (analog: `loadGlobalConfig` lines 197–200):
```go
home, err := os.UserHomeDir()
if err != nil {
    return config.FileConfig{}, fmt.Errorf("cannot determine home directory for global config: %w", err)
}
```
In a completion function this becomes silent on error:
```go
home, err := os.UserHomeDir()
if err == nil {
    sshCfgPath := filepath.Join(home, ".ssh", "config")
    hosts = append(hosts, sshconfig.ListHosts(sshCfgPath)...)
}
```

**`cmd.Root().Gen*Completion()` pattern** — use `cmd.Root()` not `cmd` because inside `RunE` the receiver is the `completion` subcommand, not the deploy root. Analog: RESEARCH.md Pitfall 4 (no codebase analog exists yet — first use of this cobra API).

---

### `internal/sshconfig/sshconfig.go` — add `ListHosts(configPath string) []string`

**Analog:** `LookupHost` in the same file (lines 41–155).

**Scanner boilerplate pattern** (analog: `LookupHost` lines 42–56):
```go
f, err := os.Open(configPath) //nolint:gosec // configPath is ~/.ssh/config, a user-controlled trusted path
if err != nil {
    return HostEntry{}, false
}
defer f.Close() //nolint:errcheck

var (
    // ...state vars...
    scanner = bufio.NewScanner(f)
)
```
`ListHosts` uses the same open/defer/scanner skeleton — returning `nil` instead of `HostEntry{}, false` on error.

**Line parsing pattern** (analog: `LookupHost` lines 57–70):
```go
for scanner.Scan() {
    line := strings.TrimSpace(scanner.Text())
    if line == "" || strings.HasPrefix(line, "#") {
        continue
    }
    parts := strings.Fields(line)
    if len(parts) < 1 {
        continue
    }
    keyword := strings.ToLower(parts[0])
    // ...keyword switch...
}
if err := scanner.Err(); err != nil {
    return HostEntry{}, false
}
```
`ListHosts` uses this exact pattern but without the `active`/`found` state machine — it only needs to detect `keyword == "host"` lines and collect `parts[1:]` aliases.

**Keyword detection pattern** (analog: `LookupHost` case `"host"` lines 83–99):
```go
case "host":
    // SSH config allows multiple patterns: "Host a b *.c"
    active = false
    for _, pattern := range parts[1:] {
        if hostMatches(pattern, alias) {
            active = true
            break
        }
    }
```
`ListHosts` iterates `parts[1:]` the same way but collects non-wildcard values instead of matching against a known alias.

**Wildcard exclusion** — use `strings.ContainsAny(pattern, "*?")` to exclude wildcard patterns. No existing analog in the file; this is new logic, but uses stdlib only.

**Silent error return** (analog: `LookupHost` line 44 open-failure return):
```go
if err != nil {
    return HostEntry{}, false
}
```
`ListHosts` returns `nil` on all error paths (file open failure, scanner error) — consistent with D-03 silent completion failure.

---

### `internal/sshconfig/sshconfig_test.go` — add `TestListHosts_*` tests

**Analog:** `TestLookupHost_*` tests in the same file (lines 14–83).

**Test structure pattern** (analog: `TestLookupHost_FoundWithAllDirectives` lines 14–45):
```go
func TestLookupHost_FoundWithAllDirectives(t *testing.T) {
    cfg := `Host minipc
  HostName 192.168.1.50
  ...
`
    tmpFile := writeTempSSHConfig(t, cfg)

    entry, found := LookupHost(tmpFile, "minipc")
    if !found {
        t.Fatal("LookupHost() returned found=false, want true")
    }
    // assertions...
}
```
Each `TestListHosts_*` test writes inline SSH config content via `writeTempSSHConfig(t, cfg)` and asserts the returned `[]string`.

**Helper function** (lines 226–239 — already exists, no change needed):
```go
func writeTempSSHConfig(t *testing.T, content string) string {
    t.Helper()
    f, err := os.CreateTemp(t.TempDir(), "ssh_config_*")
    // ...write, close, return f.Name()...
}
```
All new `TestListHosts_*` tests reuse this helper with no modification.

**Table-driven pattern** (analog: `TestFormatCheckResultFormatsAllStatusValues` in `main_test.go` lines 96–127) — use when testing multiple input variants:
```go
tests := []struct {
    name    string
    cfg     string
    want    []string
}{
    {name: "...", cfg: `...`, want: []string{...}},
}
for _, tc := range tests {
    t.Run(tc.name, func(t *testing.T) {
        // ...
    })
}
```

---

### `cmd/docker-deploy/main_test.go` — add completion subcommand and flag completion tests

**Analog:** `TestVersionCmd_Registered` (lines 130–138) and flag registration tests (lines 18–64).

**Subcommand registration test pattern** (analog: `TestVersionCmd_Registered` lines 130–138):
```go
func TestVersionCmd_Registered(t *testing.T) {
    cmd := buildDeployCmd()
    for _, sub := range cmd.Commands() {
        if sub.Use == "version" {
            return
        }
    }
    t.Fatal("deploy command has no 'version' subcommand registered")
}
```
`TestCompletionCmd_Registered` follows this exact pattern, checking for `sub.Use == "completion [bash|zsh]"`.

**Flag registration test pattern** (analog: `TestSkipEnvFlagRegistered` lines 18–27):
```go
func TestSkipEnvFlagRegistered(t *testing.T) {
    cmd := buildDeployCmd()
    f := cmd.Flags().Lookup("skip-env")
    if f == nil {
        t.Fatal("--skip-env flag not registered on deploy command")
    }
    if f.Value.Type() != "bool" {
        t.Errorf("--skip-env flag type = %q; want %q", f.Value.Type(), "bool")
    }
}
```
Flag completion registration tests use `cobra.GetFlagCompletion(cmd, "host")` (or inspect `cmd.Flag("host").Annotations`) to verify `RegisterFlagCompletionFunc` was called.

**Subcommand RunE invocation pattern** (analog: `TestVersionCmd_ExitZero` lines 235–239):
```go
func TestVersionCmd_ExitZero(t *testing.T) {
    cmd := buildVersionCmd()
    if cmd.RunE == nil {
        t.Fatal("buildVersionCmd() RunE is nil")
    }
    // ...invoke RunE...
}
```
Completion command tests capture stdout by redirecting `os.Stdout` to a `*os.File` pipe (or use `cmd.SetOut(buf)` if cobra output is set via `cmd.Root().SetOut`). Verify bash output contains `# bash completion` or `#compdef` for zsh.

---

## Shared Patterns

### Silent error handling in completion functions
**Apply to:** All three `RegisterFlagCompletionFunc` callbacks in `buildDeployCmd`
**Rule:** Any error path returns `(nil, cobra.ShellCompDirectiveNoFileComp)` — never `return err` or `panic`.
**Source:** D-03 decision in CONTEXT.md; consistent with `LookupHost`'s open-failure `return HostEntry{}, false` pattern at `sshconfig.go` line 44.

### `os.Getwd()` guard pattern
**Apply to:** `--host` and `--path` completion functions
**Source:** `runValidate` lines 152–154, `runDeploy` lines 340–341 in `cmd/docker-deploy/main.go`
```go
cwd, err := os.Getwd()
if err != nil {
    // in completion: return nil, cobra.ShellCompDirectiveNoFileComp
    // in runDeploy: return fmt.Errorf("getting working directory: %w", err)
}
```

### `os.UserHomeDir()` guard pattern
**Apply to:** `--host` completion function (for `~/.ssh/config` path)
**Source:** `loadGlobalConfig` lines 197–200 in `cmd/docker-deploy/main.go`
```go
home, err := os.UserHomeDir()
if err != nil {
    // silent in completion; error-wrapped in production paths
}
```

### `bufio.Scanner` over SSH config
**Apply to:** `ListHosts` in `internal/sshconfig/sshconfig.go`
**Source:** `LookupHost` lines 53–124 in `internal/sshconfig/sshconfig.go`
```go
scanner = bufio.NewScanner(f)
// ... scan loop ...
if err := scanner.Err(); err != nil {
    return nil  // or HostEntry{}, false in LookupHost
}
```

### `writeTempSSHConfig` test helper
**Apply to:** All new `TestListHosts_*` tests
**Source:** `internal/sshconfig/sshconfig_test.go` lines 226–239 — already exists, no change.

### `buildDeployCmd()` as test entry point
**Apply to:** All completion tests in `main_test.go`
**Source:** Every flag test in `main_test.go` starts with `cmd := buildDeployCmd()` — the same pattern applies for verifying completion registration.

---

## No Analog Found

None — all four files have strong existing analogs within the codebase.

---

## Metadata

**Analog search scope:** `cmd/docker-deploy/`, `internal/sshconfig/`, `internal/config/`
**Files scanned:** 5 (main.go, main_test.go, sshconfig.go, sshconfig_test.go, config.go)
**Pattern extraction date:** 2026-06-01
