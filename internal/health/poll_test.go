// Package health contains TDD tests for PollHealth().
// Tests use a fakeClient that implements the sessionOpener interface defined in
// poll.go, allowing deterministic control of docker ps and docker inspect output
// without requiring a real SSH connection.
//
// Time handling: tests pass Config with HealthInterval=0 (treated as 1ms internally)
// so the ticker fires quickly without blocking test runs. Timeout tests use
// HealthTimeout=1 so the timer fires within a second.
package health

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mniedre/docker-deploy/internal/config"
)

// fakeSession simulates an SSH session's Output() call.
type fakeSession struct {
	output string
	err    error
}

func (f *fakeSession) Output(_ string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []byte(f.output), nil
}

func (f *fakeSession) Close() error { return nil }

// fakeClient is a scripted sessionOpener. Each call to newSession() returns the
// next entry from the responses slice. If responses are exhausted it returns an
// error to prevent infinite loops in buggy code.
type fakeClient struct {
	responses []*fakeSession
	idx       int
}

func (f *fakeClient) newSession(_ string) (sessionOutput, error) {
	if f.idx >= len(f.responses) {
		return nil, errors.New("fakeClient: no more scripted responses")
	}
	s := f.responses[f.idx]
	f.idx++
	return s, nil
}

func newFakeClient(responses ...*fakeSession) *fakeClient {
	return &fakeClient{responses: responses}
}

// fakeSessionOut returns a fakeSession that outputs the given string.
func fakeSessionOut(output string) *fakeSession {
	return &fakeSession{output: output}
}

// defaultCfg returns a Config with given intervals for test use.
func defaultCfg(timeoutSec, intervalSec int) config.Config {
	return config.Config{
		HealthTimeout:  timeoutSec,
		HealthInterval: intervalSec,
	}
}

// --- Tests ---

// TestPollHealth_NoContainers: docker ps returns empty → PollHealth returns nil.
func TestPollHealth_NoContainers(t *testing.T) {
	// docker ps returns empty string (no containers)
	fc := newFakeClient(fakeSessionOut(""))

	err := pollHealthWithRunner(context.Background(), fc, "myproject", defaultCfg(5, 1))
	if err != nil {
		t.Fatalf("expected nil error for no containers, got: %v", err)
	}
	if fc.idx != 1 {
		t.Fatalf("expected exactly 1 session call (docker ps), got %d", fc.idx)
	}
}

// TestPollHealth_AllHealthy: two containers both return "healthy" on first poll.
func TestPollHealth_AllHealthy(t *testing.T) {
	// Sequence: docker ps → two containers; inspect c1 → healthy; inspect c2 → healthy
	fc := newFakeClient(
		fakeSessionOut("container-one\ncontainer-two\n"),
		fakeSessionOut("healthy"),
		fakeSessionOut("healthy"),
	)

	err := pollHealthWithRunner(context.Background(), fc, "myproject", defaultCfg(10, 1))
	if err != nil {
		t.Fatalf("expected nil error when all containers healthy, got: %v", err)
	}
}

// TestPollHealth_UnhealthyImmediate: one container returns "unhealthy" on first poll → non-nil error.
func TestPollHealth_UnhealthyImmediate(t *testing.T) {
	// Sequence: docker ps → one container; inspect → unhealthy
	fc := newFakeClient(
		fakeSessionOut("bad-container\n"),
		fakeSessionOut("unhealthy"),
	)

	err := pollHealthWithRunner(context.Background(), fc, "myproject", defaultCfg(10, 1))
	if err == nil {
		t.Fatal("expected non-nil error for unhealthy container, got nil")
	}
	if !strings.Contains(err.Error(), "unhealthy") {
		t.Errorf("expected error to mention 'unhealthy', got: %v", err)
	}
}

// TestPollHealth_NoHealthcheck_EmptyStatus: container returns "" for health → warning, returns nil.
func TestPollHealth_NoHealthcheck_EmptyStatus(t *testing.T) {
	fc := newFakeClient(
		fakeSessionOut("no-hc-container\n"),
		fakeSessionOut(""),
	)

	err := pollHealthWithRunner(context.Background(), fc, "myproject", defaultCfg(10, 1))
	if err != nil {
		t.Fatalf("expected nil error for no-healthcheck container (empty status), got: %v", err)
	}
}

// TestPollHealth_NoHealthcheck_NoneStatus: container returns "none" → warning, returns nil.
func TestPollHealth_NoHealthcheck_NoneStatus(t *testing.T) {
	fc := newFakeClient(
		fakeSessionOut("no-hc-container\n"),
		fakeSessionOut("none"),
	)

	err := pollHealthWithRunner(context.Background(), fc, "myproject", defaultCfg(10, 1))
	if err != nil {
		t.Fatalf("expected nil error for no-healthcheck container (none status), got: %v", err)
	}
}

// TestPollHealth_StartingThenHealthy: container "starting" twice, then "healthy" on third poll.
func TestPollHealth_StartingThenHealthy(t *testing.T) {
	// docker ps → one container
	// poll 1: starting
	// poll 2: starting
	// poll 3: healthy
	fc := newFakeClient(
		fakeSessionOut("slow-container\n"),
		fakeSessionOut("starting"),
		fakeSessionOut("starting"),
		fakeSessionOut("healthy"),
	)

	// HealthInterval=0 → 1ms effective; HealthTimeout=30s gives plenty of time
	err := pollHealthWithRunner(context.Background(), fc, "myproject", defaultCfg(30, 0))
	if err != nil {
		t.Fatalf("expected nil error after container becomes healthy, got: %v", err)
	}
}

// TestPollHealth_Timeout: container still "starting" when timeout expires → non-nil error.
func TestPollHealth_Timeout(t *testing.T) {
	// docker ps → one container that always returns "starting"
	// HealthTimeout=1 and HealthInterval=0 (1ms) so timeout fires quickly.
	responses := []*fakeSession{
		fakeSessionOut("timeout-container\n"),
	}
	for i := 0; i < 50; i++ {
		responses = append(responses, fakeSessionOut("starting"))
	}
	fc := newFakeClient(responses...)

	start := time.Now()
	err := pollHealthWithRunner(context.Background(), fc, "myproject", defaultCfg(1, 0))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected non-nil error on timeout, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected error to mention 'timed out', got: %v", err)
	}
	// Should complete within 3 seconds (1s timeout + generous buffer)
	if elapsed > 3*time.Second {
		t.Errorf("timeout test took too long: %v", elapsed)
	}
}

// TestPollHealth_Mixed: one healthy, one no-healthcheck → returns nil.
func TestPollHealth_Mixed(t *testing.T) {
	fc := newFakeClient(
		fakeSessionOut("healthy-container\nno-hc-container\n"),
		fakeSessionOut("healthy"),
		fakeSessionOut("none"),
	)

	err := pollHealthWithRunner(context.Background(), fc, "myproject", defaultCfg(10, 1))
	if err != nil {
		t.Fatalf("expected nil for mixed healthy + no-healthcheck, got: %v", err)
	}
}

// TestPollHealth_ContextCancel: ctx cancelled mid-poll → non-nil error containing "context".
func TestPollHealth_ContextCancel(t *testing.T) {
	// docker ps → one container that keeps returning "starting"
	responses := []*fakeSession{
		fakeSessionOut("container\n"),
	}
	for i := 0; i < 20; i++ {
		responses = append(responses, fakeSessionOut("starting"))
	}
	fc := newFakeClient(responses...)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := pollHealthWithRunner(ctx, fc, "myproject", defaultCfg(30, 0))
	if err == nil {
		t.Fatal("expected non-nil error on context cancel, got nil")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected error to contain 'context', got: %v", err)
	}
}
