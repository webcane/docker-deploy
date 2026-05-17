// Package health implements container health polling after docker compose up.
// PollHealth() queries docker inspect for each container in the compose project
// and reports healthy / unhealthy / unknown status, exiting non-zero on failure
// or timeout per HEALTH-01, HEALTH-02, HEALTH-03.
package health

import (
	"context"
	"fmt"

	gossh "golang.org/x/crypto/ssh"

	"github.com/mniedre/docker-deploy/internal/config"
)

// sessionOutput is the narrow interface for a single SSH session used in polls.
type sessionOutput interface {
	Output(cmd string) ([]byte, error)
	Close() error
}

// sessionOpener creates a new session for each remote command.
type sessionOpener interface {
	newSession(cmd string) (sessionOutput, error)
}

// PollHealth enumerates containers belonging to the given compose project and
// polls their health status.
// STUB: not yet implemented.
func PollHealth(ctx context.Context, client *gossh.Client, projectName string, cfg config.Config) error {
	return pollHealthWithRunner(ctx, nil, projectName, cfg)
}

// pollHealthWithRunner is the testable core of PollHealth.
// STUB: not yet implemented — returns error to make RED tests fail.
func pollHealthWithRunner(ctx context.Context, runner sessionOpener, projectName string, cfg config.Config) error {
	_ = ctx
	_ = runner
	_ = projectName
	_ = cfg
	return fmt.Errorf("not implemented")
}
