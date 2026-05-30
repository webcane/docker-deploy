// Package health contains TDD tests for PollHealth().
// Tests use a fakeClient that implements the sessionOpener interface defined in
// poll.go, allowing deterministic control of docker ps and docker inspect output
// without requiring a real SSH connection.
//
// Time handling: tests pass Config with Healthcheck.Interval=0 (treated as 1ms internally)
// so the ticker fires quickly without blocking test runs. Timeout tests use
// Healthcheck.Timeout=1s so the timer fires within a second.
package health

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/webcane/docker-deploy/internal/config"
)

// fakeSession simulates an SSH session's Output() call.
type fakeSession struct {
	output string
	err    error
}

func (f *fakeSession) Output() ([]byte, error) {
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
		Healthcheck: config.HealthcheckConfig{
			Timeout:  time.Duration(timeoutSec) * time.Second,
			Interval: time.Duration(intervalSec) * time.Second,
		},
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

// TestPollHealth_AllRunning: two containers without HEALTHCHECK both return "no-healthcheck" on first poll.
func TestPollHealth_AllRunning(t *testing.T) {
	fc := newFakeClient(
		fakeSessionOut("container-one\ncontainer-two\n"),
		fakeSessionOut("no-healthcheck"),
		fakeSessionOut("no-healthcheck"),
	)

	err := pollHealthWithRunner(context.Background(), fc, "myproject", defaultCfg(10, 1))
	if err != nil {
		t.Fatalf("expected nil error when all containers running, got: %v", err)
	}
}

// TestPollHealth_ExitedImmediate: container with HEALTHCHECK returns "unhealthy" → non-nil error.
func TestPollHealth_ExitedImmediate(t *testing.T) {
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

// TestPollHealth_DeadImmediate: container with HEALTHCHECK returns "unhealthy" immediately → non-nil error.
func TestPollHealth_DeadImmediate(t *testing.T) {
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

// TestPollHealth_CreatingThenRunning: container HEALTHCHECK in "starting" state twice, then "healthy".
func TestPollHealth_CreatingThenRunning(t *testing.T) {
	fc := newFakeClient(
		fakeSessionOut("slow-container\n"),
		fakeSessionOut("starting"),
		fakeSessionOut("starting"),
		fakeSessionOut("healthy"),
	)

	err := pollHealthWithRunner(context.Background(), fc, "myproject", defaultCfg(30, 0))
	if err != nil {
		t.Fatalf("expected nil error after container becomes healthy, got: %v", err)
	}
}

// TestPollHealth_Timeout: container HEALTHCHECK stays in "starting" state until timeout expires → non-nil error.
func TestPollHealth_Timeout(t *testing.T) {
	responses := []*fakeSession{fakeSessionOut("timeout-container\n")}
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
	if elapsed > 3*time.Second {
		t.Errorf("timeout test took too long: %v", elapsed)
	}
}

// TestPollHealth_Mixed: one container healthy immediately (no-healthcheck), one with HEALTHCHECK starting then healthy.
func TestPollHealth_Mixed(t *testing.T) {
	fc := newFakeClient(
		fakeSessionOut("fast-container\nslow-container\n"),
		fakeSessionOut("no-healthcheck"),
		fakeSessionOut("starting"),
		// second poll: fast already done; slow now healthy
		fakeSessionOut("healthy"),
	)

	err := pollHealthWithRunner(context.Background(), fc, "myproject", defaultCfg(10, 1))
	if err != nil {
		t.Fatalf("expected nil for mixed running + restarting→running, got: %v", err)
	}
}

// TestPollHealth_InspectError_ContinuesAndTimesOut: repeated inspect errors never mark container
// done, so health check times out. Inspect errors are treated as transient — keep polling.
func TestPollHealth_InspectError_ContinuesAndTimesOut(t *testing.T) {
	inspectErr := &fakeSession{err: errors.New("Process exited with status 1")}
	responses := []*fakeSession{fakeSessionOut("stuck-container\n")}
	for i := 0; i < 20; i++ {
		responses = append(responses, inspectErr)
	}
	fc := newFakeClient(responses...)

	err := pollHealthWithRunner(context.Background(), fc, "myproject", defaultCfg(1, 0))
	if err == nil {
		t.Fatal("expected non-nil error: inspect errors should not mark container done → timeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
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

// TestPollHealth_NoRetries_ImmediateFail: retries=0 (default), single unhealthy → immediate error.
// This ensures backward compat: absent retries config preserves the existing fail-fast behaviour.
func TestPollHealth_NoRetries_ImmediateFail(t *testing.T) {
	fc := newFakeClient(
		fakeSessionOut("bad-container\n"),
		fakeSessionOut("unhealthy"),
	)

	cfg := config.Config{
		Healthcheck: config.HealthcheckConfig{
			Timeout:  5 * time.Second,
			Interval: 0, // zero → 1ms test-fast
			Retries:  0, // explicit: no retries, immediate fail
		},
	}

	err := pollHealthWithRunner(context.Background(), fc, "myproject", cfg)
	if err == nil {
		t.Fatal("expected non-nil error for unhealthy container with retries=0, got nil")
	}
	if !strings.Contains(err.Error(), "unhealthy") {
		t.Errorf("expected error to mention 'unhealthy', got: %v", err)
	}
}

// TestPollHealth_RetriesThresholdReached: retries=2, container reports unhealthy on poll 1 and 2
// → PollHealth returns error after the 2nd unhealthy (failCount reaches threshold).
func TestPollHealth_RetriesThresholdReached(t *testing.T) {
	fc := newFakeClient(
		fakeSessionOut("flaky-container\n"),
		fakeSessionOut("unhealthy"), // poll 1: failCount=1, below threshold
		fakeSessionOut("unhealthy"), // poll 2: failCount=2, threshold reached → error
		// No more responses needed — should error before poll 3
	)

	cfg := config.Config{
		Healthcheck: config.HealthcheckConfig{
			Timeout:  5 * time.Second,
			Interval: 0, // zero → 1ms test-fast
			Retries:  2,
		},
	}

	err := pollHealthWithRunner(context.Background(), fc, "myproject", cfg)
	if err == nil {
		t.Fatal("expected non-nil error after retries threshold reached, got nil")
	}
	if !strings.Contains(err.Error(), "consecutive unhealthy") {
		t.Errorf("expected error to mention 'consecutive unhealthy', got: %v", err)
	}
}

// TestPollHealth_RetriesResetOnHealthy: retries=3, container reports unhealthy once then healthy.
// The single unhealthy result increments failCount but does not trip the threshold;
// healthy resets the counter; the container is eventually marked done with no error.
func TestPollHealth_RetriesResetOnHealthy(t *testing.T) {
	fc := newFakeClient(
		fakeSessionOut("recovering-container\n"),
		fakeSessionOut("unhealthy"), // poll 1: failCount=1, below threshold
		fakeSessionOut("healthy"),   // poll 2: failCount reset to 0, marked done
	)

	cfg := config.Config{
		Healthcheck: config.HealthcheckConfig{
			Timeout:  5 * time.Second,
			Interval: 0, // zero → 1ms test-fast
			Retries:  3,
		},
	}

	err := pollHealthWithRunner(context.Background(), fc, "myproject", cfg)
	if err != nil {
		t.Fatalf("expected nil error when unhealthy resets on healthy, got: %v", err)
	}
}
