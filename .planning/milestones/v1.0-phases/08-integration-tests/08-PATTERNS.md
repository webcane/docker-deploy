# Phase 8: Integration Tests - Pattern Map

**Mapped:** 2026-05-21
**Files analyzed:** 6 new files + 2 config updates (Makefile, CI workflow)
**Analogs found:** 6 / 6

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `integration/helpers_test.go` | test utility | request-response | `internal/ssh/client_test.go` | exact |
| `integration/dial_test.go` | test | request-response | `internal/ssh/client_test.go` | exact |
| `integration/preflight_test.go` | test | request-response | `internal/preflight/checks_test.go` | exact |
| `integration/filetransfer_test.go` | test | file-I/O | `internal/ssh/client_test.go` + `internal/filetransfer/upload.go` | role-match |
| `integration/compose_test.go` | test | event-driven | `internal/ssh/client_test.go` + `internal/compose/run.go` | role-match |
| `integration/testdata/Dockerfile.sshd` | config | — | no analog (new) | none |

---

## Pattern Assignments

### `integration/helpers_test.go` (test utility, request-response)

**Analog:** `internal/ssh/client_test.go`

This file holds shared container lifecycle and SSH key helpers used by all other integration test files. Copy the helper functions verbatim from the analog; expand with DinD container startup.

**Build tag + package declaration** (lines 1-3):
```go
//go:build integration

package integration_test
```

**Import pattern** — copy from `internal/ssh/client_test.go` lines 5-24, adapting the internal package import:
```go
import (
    "context"
    "crypto/rand"
    "crypto/rsa"
    "encoding/pem"
    "fmt"
    "net"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "testing"
    "time"

    gossh "golang.org/x/crypto/ssh"

    internalssh "github.com/webcane/docker-deploy/internal/ssh"
    "github.com/webcane/docker-deploy/internal/preflight"
    "github.com/webcane/docker-deploy/internal/filetransfer"
    "github.com/webcane/docker-deploy/internal/health"
    "github.com/webcane/docker-deploy/internal/compose"
    "github.com/webcane/docker-deploy/internal/config"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)
```

**sshContainer struct + startSSHContainer()** — copy verbatim from `internal/ssh/client_test.go` lines 27-87. This is Container A (SSH-only):
```go
type sshContainer struct {
    host    string
    port    int
    hostKey gossh.PublicKey
    cleanup func()
}

func startSSHContainer(t *testing.T) *sshContainer {
    t.Helper()
    ctx := context.Background()
    req := testcontainers.ContainerRequest{
        Image:        "lscr.io/linuxserver/openssh-server:latest",
        ExposedPorts: []string{"2222/tcp"},
        Env: map[string]string{
            "PUID":            "1000",
            "PGID":            "1000",
            "USER_NAME":       "testuser",
            "USER_PASSWORD":   "testpass",
            "PASSWORD_ACCESS": "true",
        },
        WaitingFor: wait.ForListeningPort("2222/tcp").WithStartupTimeout(90 * time.Second),
    }
    container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    if err != nil {
        t.Fatalf("start SSH container: %v", err)
    }
    // ... mapped port and host resolution (lines 60-86 of analog)
}
```

**captureHostKey(), seedKnownHosts(), emptyKnownHosts(), generateTestKeyFile()** — copy verbatim from `internal/ssh/client_test.go` lines 90-159. No changes required.

**startDinDContainer()** — new helper, no analog. Model on `startSSHContainer` but use `testcontainers.FromDockerfile`:
```go
type dinDContainer struct {
    host       string
    port       int
    hostKey    gossh.PublicKey
    cleanup    func()
    // keyed by username: map["sshuser"] -> signer
    signers    map[string]gossh.Signer
}

func startDinDContainer(t *testing.T) *dinDContainer {
    t.Helper()
    ctx := context.Background()
    req := testcontainers.ContainerRequest{
        FromDockerfile: testcontainers.FromDockerfile{
            Context:    "testdata/",
            Dockerfile: "Dockerfile.sshd",
            KeepImage:  false,
        },
        ExposedPorts:  []string{"22/tcp"},
        WaitingFor:    wait.ForListeningPort("22/tcp").WithStartupTimeout(120 * time.Second),
        Privileged:    true, // required for DinD
    }
    // ... same mapped port / host / captureHostKey pattern as startSSHContainer
}
```

**TestMain** — the integration package must have exactly one TestMain in helpers_test.go that starts both containers once and stores them in package-level variables:
```go
var (
    sshA   *sshContainer  // Container A: SSH-only (linuxserver/openssh-server)
    sshB   *dinDContainer // Container B: DinD+SSH (custom Dockerfile.sshd)
)

func TestMain(m *testing.M) {
    // containers started here; os.Exit(m.Run()) at the end
    // t is unavailable in TestMain — use log.Fatal for startup errors
}
```

---

### `integration/dial_test.go` (test, request-response)

**Analog:** `internal/ssh/client_test.go` lines 161-269

This file ports the four existing TestDial_* tests into the integration package against Container A (sshA). The existing tests in `internal/ssh/client_test.go` stay in place (D-05).

**Build tag + package:**
```go
//go:build integration

package integration_test
```

**Test structure** — copy the four test functions verbatim from `internal/ssh/client_test.go` lines 161-269, replacing `startSSHContainer(t)` calls with the package-level `sshA` variable (started in TestMain). The `defer sc.cleanup()` pattern is replaced with TestMain teardown:

```go
// TestDial_Timeout — copy from analog lines 162-179, no container needed
func TestDial_Timeout(t *testing.T) {
    cfg := internalssh.DialConfig{
        User:     "nobody",
        Hostname: "192.0.2.1",
        Port:     22,
        Timeout:  500 * time.Millisecond,
        Stdin:    strings.NewReader(""),
        Stdout:   os.Stderr,
    }
    _, err := internalssh.Dial(context.Background(), cfg)
    if err == nil {
        t.Fatal("expected timeout error, got nil")
    }
    if !strings.Contains(err.Error(), "timed out") {
        t.Fatalf("expected error containing 'timed out', got: %v", err)
    }
}

// TestDial_UnknownHost — uses sshA (package-level container from TestMain)
func TestDial_UnknownHost(t *testing.T) {
    cfg := internalssh.DialConfig{
        User:           "testuser",
        Hostname:       sshA.host,
        Port:           sshA.port,
        KnownHostsPath: emptyKnownHosts(t),
        Timeout:        15 * time.Second,
        Stdin:          strings.NewReader("no\n"),
        Stdout:         os.Stderr,
    }
    _, err := internalssh.Dial(context.Background(), cfg)
    if err == nil {
        t.Fatal("expected error when user rejects unknown host, got nil")
    }
}
```

**Auth injection for TestDial_Success** — Container A uses password auth; to get a successful Dial we need to inject the container's authorized key. The `generateTestKeyFile` + `seedKnownHosts` helpers from helpers_test.go provide the infrastructure. The existing analog shows that auth failure is acceptable (lines 256-268); the same approach applies here.

---

### `integration/preflight_test.go` (test, request-response)

**Analog:** `internal/preflight/checks_test.go` (entire file) + `internal/ssh/client_test.go` (container setup)

Integration preflight tests replace fakeSSHClient with a real `*gossh.Client` from Container B (sshB), injected via `preflight.NewSSHRunner()`.

**Build tag + package:**
```go
//go:build integration

package integration_test
```

**Import pattern** — no fake client needed; use real SSH runner:
```go
import (
    "bytes"
    "context"
    "os"
    "strings"
    "testing"

    "github.com/webcane/docker-deploy/internal/config"
    "github.com/webcane/docker-deploy/internal/preflight"
    internalssh "github.com/webcane/docker-deploy/internal/ssh"
    gossh "golang.org/x/crypto/ssh"
)
```

**Real SSH client construction pattern** — dial Container B with the appropriate user, then wrap with NewSSHRunner:
```go
func dialContainer(t *testing.T, user string) *gossh.Client {
    t.Helper()
    khFile := seedKnownHosts(t, sshB.host, sshB.port, sshB.hostKey)
    signer := sshB.signers[user] // pre-generated keys injected into Dockerfile
    cfg := internalssh.DialConfig{
        User:           user,
        Hostname:       sshB.host,
        Port:           sshB.port,
        KnownHostsPath: khFile,
        Timeout:        15 * time.Second,
        Stdin:          strings.NewReader(""),
        Stdout:         os.Stderr,
    }
    // Note: Dial() uses ssh-agent/config keys; for integration tests inject
    // the signer directly via a gossh.ClientConfig instead of internalssh.Dial()
    // to avoid needing a real ssh-agent socket.
    clientCfg := &gossh.ClientConfig{
        User:            user,
        Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
        HostKeyCallback: /* seeded known_hosts callback from khFile */,
        Timeout:         15 * time.Second,
    }
    client, err := gossh.Dial("tcp", fmt.Sprintf("%s:%d", sshB.host, sshB.port), clientCfg)
    if err != nil {
        t.Fatalf("dial container as %q: %v", user, err)
    }
    t.Cleanup(func() { client.Close() })
    return client
}
```

**RunPreflightChecks call pattern** — copy from `internal/preflight/checks_test.go` lines 97-108, replacing fakeSSHClient with real runner:
```go
func TestCheck01_DockerInstalled_Integration(t *testing.T) {
    client := dialContainer(t, "sshuser")
    runner := preflight.NewSSHRunner(client)
    cfg := config.Config{
        Host: config.Host{User: "sshuser", Hostname: sshB.host, Port: sshB.port},
        Path: "/opt/testapp",
    }
    _, err := preflight.RunPreflightChecks(context.Background(), runner, cfg)
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
}
```

**CHECK-07 root warning pattern** — copy from `internal/preflight/checks_test.go` lines 352-373, using captureStderr helper (copy verbatim from analog lines 81-91):
```go
func captureStderr(fn func()) string {
    r, w, _ := os.Pipe()
    old := os.Stderr
    os.Stderr = w
    fn()
    w.Close()
    os.Stderr = old
    var buf bytes.Buffer
    buf.ReadFrom(r)
    return buf.String()
}
```

**CheckResult assertion pattern** — copy from `internal/preflight/checks_test.go` lines 221-238 (finding named result in slice):
```go
var targetResult *preflight.CheckResult
for i := range results {
    if results[i].Name == "docker-group" {
        targetResult = &results[i]
        break
    }
}
if targetResult == nil {
    t.Fatal("result not found")
}
if targetResult.Status != "warn" {
    t.Errorf("expected 'warn', got %q", targetResult.Status)
}
```

---

### `integration/filetransfer_test.go` (test, file-I/O)

**Analog:** `internal/ssh/client_test.go` (container/dial setup) + `internal/filetransfer/upload.go` (Upload signature)

**Build tag + package:**
```go
//go:build integration

package integration_test
```

**Upload() signature** (from `internal/filetransfer/upload.go` line 75):
```go
func Upload(
    ctx       context.Context,
    client    *gossh.Client,
    localDir  string,
    remoteBase string,
    excludes  []string,
    sudoPw    *string,
    warnedOnce *bool,
    verbose   bool,
) (int, error)
```

**Happy-path upload call pattern:**
```go
func TestUpload_HappyPath(t *testing.T) {
    client := dialContainer(t, "sshuser")
    localDir := t.TempDir()
    // Create test files in localDir
    os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("..."), 0644)

    sudoPw := ""
    warned := false
    n, err := filetransfer.Upload(
        context.Background(),
        client,
        localDir,
        "/opt/testapp-upload",
        []string{},
        &sudoPw,
        &warned,
        false,
    )
    if err != nil {
        t.Fatalf("Upload: %v", err)
    }
    if n == 0 {
        t.Error("expected at least 1 file transferred")
    }
}
```

**Atomicity / context-cancel test pattern** (per D-12, D-13):
```go
func TestUpload_ContextCancelledMidTransfer_LeavesOriginalIntact(t *testing.T) {
    client := dialContainer(t, "sshuser")

    // Pre-seed sentinel file at the remote target.
    remoteBase := "/opt/testapp-atomic"
    sshExecHelper(t, client, fmt.Sprintf("mkdir -p %s && echo original > %s/sentinel-before-deploy.txt",
        remoteBase, remoteBase))

    // Create a local dir with enough files that cancel fires mid-transfer.
    localDir := buildLargeLocalDir(t, 20) // creates 20 small files

    ctx, cancel := context.WithCancel(context.Background())
    // Cancel after a short delay to fire mid-transfer.
    go func() {
        time.Sleep(50 * time.Millisecond)
        cancel()
    }()

    sudoPw := ""
    warned := false
    filetransfer.Upload(ctx, client, localDir, remoteBase, []string{}, &sudoPw, &warned, false) //nolint:errcheck

    // Assert: sentinel still present with original content.
    out := sshExecOutputHelper(t, client, fmt.Sprintf("cat %s/sentinel-before-deploy.txt", remoteBase))
    if strings.TrimSpace(out) != "original" {
        t.Errorf("sentinel file corrupted after cancelled upload; got %q", out)
    }
    // Assert: no staging dir remains.
    out2 := sshExecOutputHelper(t, client, "ls /tmp/docker-deploy-* 2>/dev/null || echo none")
    if !strings.Contains(out2, "none") {
        t.Errorf("staging dir not cleaned up after cancel; found: %q", out2)
    }
}
```

**sshExecHelper / sshExecOutputHelper** — inline SSH exec for test assertions. Follow the one-session-per-command rule (CLAUDE.md Rule 3):
```go
func sshExecHelper(t *testing.T, client *gossh.Client, cmd string) {
    t.Helper()
    session, err := client.NewSession()
    if err != nil {
        t.Fatalf("NewSession: %v", err)
    }
    defer session.Close()
    if err := session.Run(cmd); err != nil {
        t.Fatalf("run %q: %v", cmd, err)
    }
}

func sshExecOutputHelper(t *testing.T, client *gossh.Client, cmd string) string {
    t.Helper()
    session, err := client.NewSession()
    if err != nil {
        t.Fatalf("NewSession: %v", err)
    }
    defer session.Close()
    out, err := session.Output(cmd)
    if err != nil {
        t.Fatalf("output %q: %v", cmd, err)
    }
    return string(out)
}
```

**--skip-env test pattern** (per D-04):
```go
func TestUpload_SkipEnv_PreservesRemoteDotEnv(t *testing.T) {
    client := dialContainer(t, "sshuser")
    remoteBase := "/opt/testapp-skipenv"
    // Pre-seed .env on remote.
    sshExecHelper(t, client, fmt.Sprintf(
        "mkdir -p %s && echo 'SECRET=original' > %s/.env", remoteBase, remoteBase))

    localDir := t.TempDir()
    os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte("services: {}"), 0644)
    // .env intentionally absent from localDir — upload with ".env" in excludes

    sudoPw := ""
    warned := false
    _, err := filetransfer.Upload(
        context.Background(), client, localDir, remoteBase,
        []string{".env"}, &sudoPw, &warned, false,
    )
    if err != nil {
        t.Fatalf("Upload with skip-env: %v", err)
    }

    out := sshExecOutputHelper(t, client, fmt.Sprintf("cat %s/.env", remoteBase))
    if strings.TrimSpace(out) != "SECRET=original" {
        t.Errorf("remote .env was changed; got %q", out)
    }
}
```

---

### `integration/compose_test.go` (test, event-driven)

**Analog:** `internal/ssh/client_test.go` (container setup) + `internal/compose/run.go` (RunCompose signature) + `internal/health/poll.go` (PollHealth signature)

**Build tag + package:**
```go
//go:build integration

package integration_test
```

**RunCompose() signature** (from `internal/compose/run.go` line 51):
```go
func RunCompose(ctx context.Context, client *gossh.Client, remotePath, composeFile string, verbose bool) error
```

**PollHealth() signature** (from `internal/health/poll.go` line 92):
```go
func PollHealth(ctx context.Context, client *gossh.Client, projectName string, cfg config.Config) error
```

**Full E2E compose + health test pattern:**
```go
func TestCompose_HappyPath_Nginx(t *testing.T) {
    client := dialContainer(t, "sshuser")
    remoteBase := "/opt/compose-test-nginx"

    // 1. Upload compose.yaml with nginx:alpine service (no HEALTHCHECK).
    localDir := t.TempDir()
    os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte(`
services:
  web:
    image: nginx:alpine
    ports: ["80"]
`), 0644)
    sudoPw := ""
    warned := false
    _, err := filetransfer.Upload(context.Background(), client, localDir, remoteBase,
        []string{}, &sudoPw, &warned, false)
    if err != nil {
        t.Fatalf("Upload: %v", err)
    }

    // 2. Run compose up.
    if err := compose.RunCompose(context.Background(), client, remoteBase, "compose.yaml", false); err != nil {
        t.Fatalf("RunCompose: %v", err)
    }

    // 3. Poll health — nginx:alpine has no HEALTHCHECK, so "none" status is expected (HEALTH-02).
    cfg := config.Config{HealthTimeout: 30, HealthInterval: 2}
    if err := health.PollHealth(context.Background(), client, "compose-test-nginx", cfg); err != nil {
        t.Fatalf("PollHealth: %v", err)
    }

    // 4. Cleanup: docker compose down.
    t.Cleanup(func() {
        sshExecHelper(t, client,
            fmt.Sprintf("docker compose -f %s/compose.yaml down --remove-orphans 2>/dev/null || true",
                remoteBase))
    })
}
```

**Unhealthy HEALTHCHECK test** (per D-03):
```go
func TestCompose_UnhealthyService_ReturnsError(t *testing.T) {
    client := dialContainer(t, "sshuser")
    remoteBase := "/opt/compose-test-unhealthy"

    // HEALTHCHECK that always fails: curl to a closed port.
    localDir := t.TempDir()
    os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte(`
services:
  web:
    image: nginx:alpine
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9999"]
      interval: 2s
      timeout: 1s
      retries: 1
      start_period: 0s
`), 0644)
    // ... upload + compose up + PollHealth; assert non-nil error containing "unhealthy"
}
```

**Project name derivation** — compose project name defaults to the directory basename (Docker Compose convention). Use a predictable name matching the `remoteBase` directory basename for label-based enumeration in PollHealth.

---

### `integration/testdata/Dockerfile.sshd` (config)

**Analog:** None — first DinD+SSH Dockerfile in this project.

**Base image choice** (per D-09, D-11 specifics): `ubuntu:22.04` base with Docker Engine CE and OpenSSH daemon installed manually gives maximum control over user setup. Alternatively `docker:dind` + OpenSSH; ubuntu:22.04 is preferred per CONTEXT.md §Specific Ideas.

**Four users required** (D-10):
| Username | sudoers | Purpose |
|---|---|---|
| `root` | (already root) | CHECK-07 root-user warning |
| `nosudouser` | none | Failing path for CHECK-04/CHECK-06 |
| `sudopassuser` | sudo with password required | DEPLOY-07 fallback path |
| `sshuser` | `NOPASSWD: ALL` | Happy path (passwordless sudo) |

**Key structural requirements:**
- Each user must have an `authorized_keys` file populated with a test key baked into the image (generated at build time via `RUN ssh-keygen` or injected as a build ARG).
- Docker daemon must be running (via `CMD ["sh", "-c", "dockerd & /usr/sbin/sshd -D"]` or supervisor).
- Port 22 exposed.
- `PermitRootLogin yes` in sshd_config (for root user test).

---

## Shared Patterns

### Build Tag — Apply to All `integration/` Files
**Source:** `internal/ssh/client_test.go` lines 1-3
```go
//go:build integration

package integration_test
```
Every file in `integration/` must have this exact build tag as lines 1-2 (blank line between tag and package declaration). Package name is always `integration_test` (external test package per D-06).

### TestMain Lifecycle — `integration/helpers_test.go`
**Apply to:** `helpers_test.go` only (one TestMain per package)

```go
func TestMain(m *testing.M) {
    // Start Container A and Container B.
    // Store in package-level vars sshA, sshB.
    // os.Exit(m.Run()) at end.
    // Teardown via defer before os.Exit.
}
```
Note: `t.Helper()` is unavailable in TestMain — use `log.Fatal` or `fmt.Fprintf(os.Stderr, ...) + os.Exit(1)` for startup errors.

### One NewSession Per Command — All Test Helper SSH Calls
**Source:** `internal/filetransfer/upload.go` lines 386-397 (sshExec pattern); CLAUDE.md Rule 3
**Apply to:** All `sshExecHelper`, `sshExecOutputHelper`, `dialContainer` helpers

```go
session, err := client.NewSession()
if err != nil { t.Fatalf("NewSession: %v", err) }
defer session.Close()
// one Run() or Output() call only
```

### No InsecureIgnoreHostKey — All Dial Calls
**Source:** CLAUDE.md Rule 1; `internal/ssh/client_test.go` captureHostKey pattern (lines 90-109)
**Apply to:** All places that construct `gossh.ClientConfig` in integration tests

Always use a real known-hosts callback. The pattern is: `captureHostKey()` to get the server key → `seedKnownHosts()` → build `knownhosts.New()` callback from the seeded file. Never use `gossh.InsecureIgnoreHostKey()`.

### Config Construction for Internal API Calls
**Source:** `internal/preflight/checks_test.go` lines 74-79 (defaultCfg helper)
**Apply to:** `preflight_test.go`, `compose_test.go`, `filetransfer_test.go`

Integration tests construct `config.Config` directly — no CLI flags, no deploy.yaml loading:
```go
cfg := config.Config{
    Host:           config.Host{User: "sshuser", Hostname: sshB.host, Port: sshB.port},
    Path:           "/opt/testapp",
    HealthTimeout:  30,
    HealthInterval: 2,
}
```

### testcontainers Container Startup Pattern
**Source:** `internal/ssh/client_test.go` lines 39-56
**Apply to:** `startSSHContainer()` and `startDinDContainer()` in helpers_test.go

```go
container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
    ContainerRequest: req,
    Started:          true,
})
// Always use WaitingFor: wait.ForListeningPort("22/tcp").WithStartupTimeout(90 * time.Second)
// DinD needs 120s startup timeout due to Docker daemon initialisation.
```

### captureStderr Helper
**Source:** `internal/preflight/checks_test.go` lines 81-91
**Apply to:** `preflight_test.go` (CHECK-03 daemon warning, CHECK-07 root warning tests)

```go
func captureStderr(fn func()) string {
    r, w, _ := os.Pipe()
    old := os.Stderr
    os.Stderr = w
    fn()
    w.Close()
    os.Stderr = old
    var buf bytes.Buffer
    buf.ReadFrom(r)
    return buf.String()
}
```

### gossh.ExitError for Exit-Code Assertions
**Source:** `internal/preflight/checks_test.go` lines 47-52
**Apply to:** `compose_test.go` (HEALTH-03 exit code propagation test)

```go
var exitErr *gossh.ExitError
if errors.As(err, &exitErr) {
    // exitErr.ExitStatus() gives the remote exit code
}
```

---

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `integration/testdata/Dockerfile.sshd` | config | — | No DinD+SSH Dockerfile exists in the project; no analog possible |
| `integration/compose_test.go` (health polling integration) | test | event-driven | Existing health tests (`internal/health/poll_test.go` if present) use fakes; no real-container health poll test exists yet |

---

## Key Observations for Planner

1. **Four-user key injection**: The integration test suite needs per-user RSA keys baked into `Dockerfile.sshd`. The `generateTestKeyFile()` helper produces keys at test time; for the Dockerfile these keys must be generated at image build time (via `RUN ssh-keygen -t rsa -f /etc/ssh/test_keys/<user>_rsa -N ''`) and the public key written to each user's `authorized_keys`. The test suite reads the private key back out of a well-known path inside the container (via SFTP or an initial SSH exec) to construct its signers.

2. **internalssh.Dial() vs raw gossh.Dial()**: `internalssh.Dial()` uses `buildAuthMethods()` which reads `~/.ssh/config` and `$SSH_AUTH_SOCK`. In CI these are not available. Integration tests should dial containers using `gossh.Dial()` directly with an explicit `gossh.PublicKeys(signer)` auth method, bypassing the agent chain. The `internalssh.Dial()` function is still tested via `dial_test.go` for the TOFU/timeout paths (which don't need auth success).

3. **Makefile target**: `test-integration: go test -tags integration -timeout 5m ./integration/...` — no analog exists; add alongside existing `test` target.

4. **CI workflow**: New `integration` job after `test` job on `ubuntu-latest` — no existing workflow file was scanned; planner must identify the current workflow file path (likely `.github/workflows/ci.yml`).

---

## Metadata

**Analog search scope:** `internal/ssh/`, `internal/preflight/`, `internal/filetransfer/`, `internal/health/`, `internal/compose/`, `internal/config/`
**Files scanned:** 7 source files
**Pattern extraction date:** 2026-05-21
