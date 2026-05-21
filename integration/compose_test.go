//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/webcane/docker-deploy/internal/compose"
	"github.com/webcane/docker-deploy/internal/config"
	"github.com/webcane/docker-deploy/internal/filetransfer"
	"github.com/webcane/docker-deploy/internal/health"
)

// composeHealthyYAML is nginx:alpine with no HEALTHCHECK. The container starts
// quickly, reaches "running" state, and PollHealth returns nil (HEALTH-01/HEALTH-02).
// PollHealth polls {{.State.Status}} — "running" is the terminal-success state.
const composeHealthyYAML = `services:
  web:
    image: nginx:alpine
    ports:
      - "80"
`

// composeUnhealthyYAML uses busybox with "exit 1" as the command so the container
// exits immediately. PollHealth detects state="exited" and returns a non-nil error
// (HEALTH-03: deploy fails when containers stop unexpectedly).
// NOTE: poll.go checks {{.State.Status}} (running/exited/dead), not HEALTHCHECK status.
// A container that exits is the reliable way to trigger PollHealth's error path.
const composeUnhealthyYAML = `services:
  web:
    image: busybox
    command: ["sh", "-c", "exit 1"]
    restart: "no"
`

// TestCompose_Healthy_NoHealthcheck verifies that a compose project running nginx:alpine
// (no HEALTHCHECK, container stays "running") causes PollHealth to return nil (HEALTH-01/HEALTH-02).
// The healthy path: container reaches running state → PollHealth returns nil.
func TestCompose_Healthy_NoHealthcheck(t *testing.T) {
	client := dialContainer(t, "sshuser")
	remoteBase := "/opt/compose-test-healthy"

	// Write compose YAML to a temp dir and upload it to the remote.
	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte(composeHealthyYAML), 0644); err != nil {
		t.Fatalf("write compose.yaml: %v", err)
	}

	sudoPw := ""
	warned := false
	if _, err := filetransfer.Upload(context.Background(), client, localDir, remoteBase,
		[]string{}, &sudoPw, &warned, false); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	// Run docker compose up -d.
	if err := compose.RunCompose(context.Background(), client, remoteBase, "compose.yaml", false); err != nil {
		t.Fatalf("RunCompose: %v", err)
	}

	// PollHealth should return nil — nginx:alpine container reaches "running" state.
	// project name = directory basename of remoteBase (Docker Compose convention).
	cfg := config.Config{HealthTimeout: 30, HealthInterval: 2}
	if err := health.PollHealth(context.Background(), client, "compose-test-healthy", cfg); err != nil {
		t.Fatalf("PollHealth: unexpected error for healthy service: %v", err)
	}

	// Cleanup: bring down containers to avoid dirty state for subsequent tests (D-07).
	t.Cleanup(func() {
		sshExecHelper(t, client,
			fmt.Sprintf("docker compose -f %s/compose.yaml down --remove-orphans 2>/dev/null || true",
				remoteBase))
	})
}

// TestCompose_Unhealthy_ReturnError verifies that a compose project whose container
// exits immediately causes PollHealth to return a non-nil error (HEALTH-03).
// poll.go checks {{.State.Status}} — "exited" or "dead" triggers the error return.
// The error message contains "stopped unexpectedly".
func TestCompose_Unhealthy_ReturnError(t *testing.T) {
	client := dialContainer(t, "sshuser")
	remoteBase := "/opt/compose-test-unhealthy"

	// Write compose YAML (busybox, exits immediately) and upload it.
	localDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(localDir, "compose.yaml"), []byte(composeUnhealthyYAML), 0644); err != nil {
		t.Fatalf("write compose.yaml: %v", err)
	}

	sudoPw := ""
	warned := false
	if _, err := filetransfer.Upload(context.Background(), client, localDir, remoteBase,
		[]string{}, &sudoPw, &warned, false); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	// docker compose up -d succeeds even if the service will immediately exit.
	if err := compose.RunCompose(context.Background(), client, remoteBase, "compose.yaml", false); err != nil {
		t.Fatalf("RunCompose: unexpected error: %v", err)
	}

	// Allow a short pause for the container to exit before polling.
	// PollHealth polls at 2s interval with 30s timeout; the container exits
	// almost immediately so the first poll will see it.
	cfg := config.Config{HealthTimeout: 30, HealthInterval: 2}
	err := health.PollHealth(context.Background(), client, "compose-test-unhealthy", cfg)

	// Assert non-nil error (HEALTH-03).
	if err == nil {
		t.Fatal("expected non-nil error from PollHealth for exited service, got nil")
	}

	// The error should indicate the container stopped unexpectedly (poll.go line 213).
	// poll.go uses {{.State.Status}} — "exited" or "dead" triggers this message.
	if !strings.Contains(err.Error(), "stopped unexpectedly") {
		t.Errorf("expected error containing 'stopped unexpectedly', got: %v", err)
	}

	// Cleanup: bring down to avoid dirty state for subsequent tests (D-07).
	t.Cleanup(func() {
		sshExecHelper(t, client,
			fmt.Sprintf("docker compose -f %s/compose.yaml down --remove-orphans 2>/dev/null || true",
				remoteBase))
	})
}

// TestHealth_NoContainers verifies that PollHealth returns nil when the compose
// project has no running containers (HEALTH-01: empty project = nothing to poll).
// This exercises the len(containers) == 0 early-return path in pollHealthWithRunner.
func TestHealth_NoContainers(t *testing.T) {
	client := dialContainer(t, "sshuser")

	// Use a project name that does not exist in the DinD Docker daemon.
	// docker ps --filter label=com.docker.compose.project=nonexistent-project-xyz
	// returns empty output → listContainers returns [] → PollHealth returns nil.
	cfg := config.Config{HealthTimeout: 10, HealthInterval: 1}
	if err := health.PollHealth(context.Background(), client, "nonexistent-project-xyz", cfg); err != nil {
		t.Fatalf("PollHealth for empty project: unexpected error: %v", err)
	}
}
