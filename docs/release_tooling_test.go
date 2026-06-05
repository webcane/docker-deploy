package docs_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// mustReadTooling reads a file relative to the project root (one level up from docs/).
func mustReadTooling(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile("../" + rel) //nolint:gosec // G304: path is a test-time constant relative to project root, not user input
	if err != nil {
		t.Fatalf("cannot read %s: %v", rel, err)
	}
	return string(data)
}

// TestSC161_UnitTestsHardAbort verifies that the release skill runs go test ./... before
// the version bump question (Wave 0 Step A present before Step 1; ABORT message present) (SC-16-1).
func TestSC161_UnitTestsHardAbort(t *testing.T) { //nolint:gocognit // test function: subtests are flat checks, complexity is inherent to multi-assertion tests
	content := mustReadTooling(t, ".claude/commands/gsd/release-tag.md")

	t.Run("Wave 0 section exists before Step 1", func(t *testing.T) {
		wave0Idx := strings.Index(content, "Wave 0")
		step1Idx := strings.Index(content, "## Step 1")
		if wave0Idx == -1 {
			t.Fatal("release-tag.md must contain a 'Wave 0' section")
		}
		if step1Idx == -1 {
			t.Fatal("release-tag.md must contain '## Step 1'")
		}
		if wave0Idx >= step1Idx {
			t.Errorf("Wave 0 section (pos %d) must appear BEFORE Step 1 (pos %d) in release-tag.md", wave0Idx, step1Idx)
		}
	})

	t.Run("go test command invoked via make test", func(t *testing.T) {
		if !strings.Contains(content, "make test") {
			t.Error("release-tag.md Wave 0 must invoke 'make test' for unit tests")
		}
	})

	t.Run("ABORT message present for unit test failure", func(t *testing.T) {
		if !strings.Contains(content, "ABORT: unit tests failed") {
			t.Error("release-tag.md must print 'ABORT: unit tests failed' when go test exits non-zero")
		}
	})

	t.Run("go test arrow prefix present", func(t *testing.T) {
		if !strings.Contains(content, "▶ go test ./...") {
			t.Error("release-tag.md must print '▶ go test ./...' before running unit tests")
		}
	})
}

// TestSC162_TestCIDockerAutoDetect verifies that the release skill detects Docker socket,
// skips integration tests with a warning when Docker is absent, and aborts if Docker is present
// and tests fail (SC-16-2).
func TestSC162_TestCIDockerAutoDetect(t *testing.T) {
	content := mustReadTooling(t, ".claude/commands/gsd/release-tag.md")

	t.Run("Docker socket detection string present", func(t *testing.T) {
		// Both paths must be checked as per D-04
		if !strings.Contains(content, "/var/run/docker.sock") {
			t.Error("release-tag.md must check for /var/run/docker.sock to detect Docker")
		}
		if !strings.Contains(content, ".colima/default/docker.sock") {
			t.Error("release-tag.md must check for $HOME/.colima/default/docker.sock (colima support)")
		}
	})

	t.Run("WARNING skip path present when Docker not detected", func(t *testing.T) {
		if !strings.Contains(content, "WARNING: Docker not detected") {
			t.Error("release-tag.md must print 'WARNING: Docker not detected' when no Docker socket found")
		}
	})

	t.Run("ABORT path present when integration tests fail", func(t *testing.T) {
		if !strings.Contains(content, "ABORT: integration tests failed") {
			t.Error("release-tag.md must print 'ABORT: integration tests failed' when make test-ci fails with Docker present")
		}
	})

	t.Run("make test-ci invoked for integration tests", func(t *testing.T) {
		if !strings.Contains(content, "make test-ci") {
			t.Error("release-tag.md must invoke 'make test-ci' for integration tests")
		}
	})

	t.Run("integration test arrow prefix present", func(t *testing.T) {
		if !strings.Contains(content, "▶ test-ci (integration tests)") {
			t.Error("release-tag.md must print '▶ test-ci (integration tests)' before running integration tests")
		}
	})
}

// TestSC163_LintRunsAndLintFixApplied verifies that the release skill runs make lint and
// applies make lint-fix when issues are found (Wave 0 Step B) (SC-16-3).
func TestSC163_LintRunsAndLintFixApplied(t *testing.T) {
	content := mustReadTooling(t, ".claude/commands/gsd/release-tag.md")

	t.Run("make lint present in Wave 0", func(t *testing.T) {
		if !strings.Contains(content, "make lint") {
			t.Error("release-tag.md Wave 0 must invoke 'make lint'")
		}
	})

	t.Run("make lint-fix present in Wave 0", func(t *testing.T) {
		if !strings.Contains(content, "make lint-fix") {
			t.Error("release-tag.md Wave 0 must invoke 'make lint-fix' to auto-fix lint issues")
		}
	})

	t.Run("lint arrow prefix present", func(t *testing.T) {
		if !strings.Contains(content, "▶ golangci-lint run ./...") {
			t.Error("release-tag.md must print '▶ golangci-lint run ./...' before running linter")
		}
	})

	t.Run("make lint-fix appears after make lint failure branch", func(t *testing.T) {
		lintIdx := strings.Index(content, "make lint")
		lintFixIdx := strings.Index(content, "make lint-fix")
		if lintFixIdx == -1 {
			t.Fatal("make lint-fix not found in release-tag.md")
		}
		if lintFixIdx <= lintIdx {
			t.Errorf("make lint-fix (pos %d) must appear AFTER the initial make lint (pos %d) as a recovery step", lintFixIdx, lintIdx)
		}
	})
}

// TestSC164_SecondLintRunAfterFix verifies that the release skill re-runs lint after auto-fix
// and only aborts on the second failure, not the first (SC-16-4).
func TestSC164_SecondLintRunAfterFix(t *testing.T) {
	content := mustReadTooling(t, ".claude/commands/gsd/release-tag.md")

	t.Run("retry gate: make lint appears at least twice (initial + retry)", func(t *testing.T) {
		count := strings.Count(content, "make lint")
		// make lint must appear at minimum twice: initial run and retry after lint-fix
		if count < 2 {
			t.Errorf("release-tag.md must invoke 'make lint' at least twice (initial run + retry after lint-fix); found occurrences: %d", count)
		}
	})

	t.Run("lint retry appears after lint-fix", func(t *testing.T) {
		lintFixIdx := strings.Index(content, "make lint-fix")
		if lintFixIdx == -1 {
			t.Fatal("make lint-fix not found")
		}
		afterFix := content[lintFixIdx:]
		if !strings.Contains(afterFix, "make lint") {
			t.Error("release-tag.md must re-run 'make lint' after 'make lint-fix' to verify fixes")
		}
	})

	t.Run("ABORT only on persistent lint failure (after retry)", func(t *testing.T) {
		// The ABORT for lint must be positioned after make lint-fix in the file
		lintFixIdx := strings.Index(content, "make lint-fix")
		lintAbortIdx := strings.Index(content, "ABORT: lint issues remain after auto-fix")
		if lintAbortIdx == -1 {
			t.Fatal("release-tag.md must contain 'ABORT: lint issues remain after auto-fix'")
		}
		if lintAbortIdx <= lintFixIdx {
			t.Errorf("lint ABORT message (pos %d) must appear AFTER make lint-fix (pos %d) — abort only on second failure", lintAbortIdx, lintFixIdx)
		}
	})

	t.Run("PASS auto-fixed message present for recovery path", func(t *testing.T) {
		if !strings.Contains(content, "PASS (auto-fixed)") {
			t.Error("release-tag.md must print 'PASS (auto-fixed)' when second lint run passes")
		}
	})
}

func checkStateMDLastUpdated(t *testing.T, content string) {
	t.Helper()
	if !strings.Contains(content, "last_updated:") {
		t.Error("release-tag.md must instruct updating 'last_updated:' in STATE.md")
	}
	isoCmd := `date -u +"%Y-%m-%dT%H:%M:%SZ"`
	if !strings.Contains(content, isoCmd) {
		t.Error("release-tag.md must use the ISO 8601 date command (date -u) to generate the last_updated timestamp")
	}
}

func checkStateMDLastActivity(t *testing.T, content string) {
	t.Helper()
	if !strings.Contains(content, "last_activity:") {
		t.Error("release-tag.md must instruct updating 'last_activity:' in STATE.md")
	}
	if !strings.Contains(content, "Released") {
		t.Error("release-tag.md last_activity update must contain 'Released' pattern (e.g. '2026-05-27 -- Released v0.9.4')")
	}
}

func checkStateMDGitAdd(t *testing.T, content string) {
	t.Helper()
	if !strings.Contains(content, ".planning/STATE.md") {
		t.Error("release-tag.md must include '.planning/STATE.md' in the git add command")
	}
	hasGitAdd := false
	for _, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "git add") && strings.Contains(line, "STATE.md") {
			hasGitAdd = true
			break
		}
	}
	if !hasGitAdd {
		t.Error("release-tag.md must stage '.planning/STATE.md' via 'git add ... .planning/STATE.md'")
	}
}

func checkStateMDMilestoneGuard(t *testing.T, content string) {
	t.Helper()
	if !strings.Contains(content, "milestone:") {
		t.Error("release-tag.md must mention 'milestone:' to explicitly instruct NOT changing it")
	}
	hasMilestoneGuard := strings.Contains(content, "NOT change `milestone:`") ||
		strings.Contains(content, "Do NOT change `milestone:`") ||
		strings.Contains(content, "Do **NOT** change `milestone:`")
	if !hasMilestoneGuard {
		t.Error("release-tag.md must explicitly say NOT to change 'milestone:' field in STATE.md")
	}
}

// TestSC165_StateMDUpdatedWithVersion verifies that the release skill updates STATE.md with
// version and release date in ISO 8601 format, staged in the same git add (SC-16-5).
func TestSC165_StateMDUpdatedWithVersion(t *testing.T) {
	content := mustReadTooling(t, ".claude/commands/gsd/release-tag.md")

	t.Run("last_updated ISO 8601 timestamp instruction present", func(t *testing.T) {
		checkStateMDLastUpdated(t, content)
	})

	t.Run("last_activity with Released pattern present", func(t *testing.T) {
		checkStateMDLastActivity(t, content)
	})

	t.Run(".planning/STATE.md in git add", func(t *testing.T) {
		checkStateMDGitAdd(t, content)
	})

	t.Run("milestone field explicitly NOT changed", func(t *testing.T) {
		checkStateMDMilestoneGuard(t, content)
	})
}

// TestSC166_CommitBodyFromGitLog verifies that the release skill derives the commit body
// from git log (non-chore commits only) with the correct format (SC-16-6).
func TestSC166_CommitBodyFromGitLog(t *testing.T) {
	content := mustReadTooling(t, ".claude/commands/gsd/release-tag.md")

	t.Run("git log $PREV_TAG..HEAD --oneline command present", func(t *testing.T) {
		if !strings.Contains(content, "git log $PREV_TAG..HEAD --oneline") {
			t.Error("release-tag.md must use 'git log $PREV_TAG..HEAD --oneline' to derive commit body")
		}
	})

	t.Run("chore: filter logic present", func(t *testing.T) {
		// Must filter out chore: and chore( prefixes
		hasChoreFilter := strings.Contains(content, "chore:") && strings.Contains(content, "chore(")
		if !hasChoreFilter {
			t.Error("release-tag.md must filter out lines starting with 'chore:' or 'chore(' from commit body")
		}
	})

	t.Run("Changes since pattern present in commit body format", func(t *testing.T) {
		if !strings.Contains(content, "Changes since $CURRENT_TAG:") {
			t.Error("release-tag.md must use 'Changes since $CURRENT_TAG:' as the commit body header")
		}
	})

	t.Run("empty body falls back to subject-only commit", func(t *testing.T) {
		hasEmptyBodyPath := strings.Contains(content, "BODY_LINES is empty") ||
			strings.Contains(content, "$BODY_LINES` is empty") ||
			strings.Contains(content, "If `$BODY_LINES` is empty")
		if !hasEmptyBodyPath {
			t.Error("release-tag.md must document the empty-body path (subject-only commit when all commits are chores)")
		}
	})

	t.Run("bullet format with dash prefix used for commit lines", func(t *testing.T) {
		if !strings.Contains(content, `sed 's/^/- /'`) && !strings.Contains(content, "- {line") {
			t.Error("release-tag.md commit body must prefix each git log line with '- ' (bullet format)")
		}
	})
}

// TestSC1610_GosecEnabled verifies that gosec, ineffassign, unused, bodyclose, and noctx
// are all enabled in .golangci.yml (SC-16-10).
func TestSC1610_GosecEnabled(t *testing.T) {
	content := mustReadTooling(t, ".golangci.yml")

	linters := []string{"gosec", "ineffassign", "unused", "bodyclose", "noctx"}
	for _, linter := range linters {
		linter := linter
		t.Run(linter+" in linters.enable", func(t *testing.T) {
			if !strings.Contains(content, "- "+linter) {
				t.Errorf(".golangci.yml must have '- %s' in linters.enable section", linter)
			}
		})
	}
}

// TestSC1611_QualityLintersEnabled verifies that gocritic, revive, errorlint, and wrapcheck
// are all enabled in .golangci.yml (SC-16-11).
func TestSC1611_QualityLintersEnabled(t *testing.T) {
	content := mustReadTooling(t, ".golangci.yml")

	linters := []string{"gocritic", "revive", "errorlint", "wrapcheck"}
	for _, linter := range linters {
		linter := linter
		t.Run(linter+" in linters.enable", func(t *testing.T) {
			if !strings.Contains(content, "- "+linter) {
				t.Errorf(".golangci.yml must have '- %s' in linters.enable section", linter)
			}
		})
	}
}

func checkGocognitThreshold(t *testing.T, content string) {
	t.Helper()
	gocognitIdx := strings.Index(content, "gocognit:")
	if gocognitIdx == -1 {
		t.Fatal(".golangci.yml must have a gocognit settings block")
	}
	afterGocognit := content[gocognitIdx:]
	windowEnd := 200
	if len(afterGocognit) < windowEnd {
		windowEnd = len(afterGocognit)
	}
	if !strings.Contains(afterGocognit[:windowEnd], "min-complexity: 15") {
		t.Error(".golangci.yml gocognit settings must contain 'min-complexity: 15'")
	}
}

func checkNestifThreshold(t *testing.T, content string) {
	t.Helper()
	nestifIdx := strings.Index(content, "nestif:")
	if nestifIdx == -1 {
		t.Fatal(".golangci.yml must have a nestif settings block")
	}
	afterNestif := content[nestifIdx:]
	windowEnd := 200
	if len(afterNestif) < windowEnd {
		windowEnd = len(afterNestif)
	}
	if !strings.Contains(afterNestif[:windowEnd], "min-complexity: 5") {
		t.Error(".golangci.yml nestif settings must contain 'min-complexity: 5'")
	}
}

// TestSC1612_ComplexityThresholds verifies that gocognit has min-complexity: 15 and
// nestif has min-complexity: 5 configured (SC-16-12).
func TestSC1612_ComplexityThresholds(t *testing.T) {
	content := mustReadTooling(t, ".golangci.yml")

	t.Run("gocognit in linters.enable", func(t *testing.T) {
		if !strings.Contains(content, "- gocognit") {
			t.Error(".golangci.yml must have '- gocognit' in linters.enable")
		}
	})

	t.Run("nestif in linters.enable", func(t *testing.T) {
		if !strings.Contains(content, "- nestif") {
			t.Error(".golangci.yml must have '- nestif' in linters.enable")
		}
	})

	t.Run("gocognit min-complexity is 15", func(t *testing.T) {
		checkGocognitThreshold(t, content)
	})

	t.Run("nestif min-complexity is 5", func(t *testing.T) {
		checkNestifThreshold(t, content)
	})
}

// TestSC1613_PreallocEnabled verifies that prealloc is in linters.enable (SC-16-13).
func TestSC1613_PreallocEnabled(t *testing.T) {
	content := mustReadTooling(t, ".golangci.yml")

	t.Run("prealloc in linters.enable", func(t *testing.T) {
		if !strings.Contains(content, "- prealloc") {
			t.Error(".golangci.yml must have '- prealloc' in linters.enable")
		}
	})
}

// TestSC1614_ErrcheckExcludes verifies that errcheck excludes fmt.Fprintf, fmt.Fprintln,
// fmt.Fprint, and SSH/SFTP .Close() calls (SC-16-14).
func TestSC1614_ErrcheckExcludes(t *testing.T) {
	content := mustReadTooling(t, ".golangci.yml")

	excludes := []struct {
		name    string
		pattern string
	}{
		{"fmt.Fprintf", "fmt.Fprintf"},
		{"fmt.Fprintln", "fmt.Fprintln"},
		{"fmt.Fprint", "fmt.Fprint"},
		{"ssh.Client.Close", "ssh.Client).Close"},
		{"ssh.Session.Close", "ssh.Session).Close"},
		{"sftp.Client.Close", "sftp.Client).Close"},
	}

	for _, exc := range excludes {
		exc := exc
		t.Run(exc.name+" in errcheck.exclude-functions", func(t *testing.T) {
			if !strings.Contains(content, exc.pattern) {
				t.Errorf(".golangci.yml errcheck.exclude-functions must exclude %q (pattern %q)", exc.name, exc.pattern)
			}
		})
	}
}

// TestSC1615_WrapcheckIgnoreSigs verifies that wrapcheck.ignore-sigs covers .Errorf(,
// errors.New(, and errors.Unwrap( (SC-16-15).
func TestSC1615_WrapcheckIgnoreSigs(t *testing.T) {
	content := mustReadTooling(t, ".golangci.yml")

	t.Run("wrapcheck settings block present", func(t *testing.T) {
		if !strings.Contains(content, "wrapcheck:") {
			t.Fatal(".golangci.yml must have a wrapcheck settings block")
		}
	})

	t.Run("ignore-sigs block present", func(t *testing.T) {
		if !strings.Contains(content, "ignore-sigs:") {
			t.Error(".golangci.yml wrapcheck settings must contain 'ignore-sigs:'")
		}
	})

	sigs := []struct {
		name    string
		pattern string
	}{
		{".Errorf(", ".Errorf("},
		{"errors.New(", "errors.New("},
		{"errors.Unwrap(", "errors.Unwrap("},
	}

	for _, sig := range sigs {
		sig := sig
		t.Run(sig.name+" in wrapcheck.ignore-sigs", func(t *testing.T) {
			if !strings.Contains(content, sig.pattern) {
				t.Errorf(".golangci.yml wrapcheck.ignore-sigs must include %q", sig.pattern)
			}
		})
	}
}

// TestSC1616_MakeLintExitsZero runs make lint from the project root and verifies it exits 0.
// Skips if golangci-lint is not installed (SC-16-16).
func TestSC1616_MakeLintExitsZero(t *testing.T) {
	// Check if golangci-lint is installed; skip gracefully if not
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not installed — skipping make lint execution test")
	}

	// Confirm make is available
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make not installed — skipping make lint execution test")
	}

	t.Run("make lint exits 0 from project root", func(t *testing.T) {
		cmd := exec.CommandContext(context.Background(), "make", "lint") //nolint:gosec // G204: make lint is a fixed hardcoded command, not user input
		cmd.Dir = ".."                                                   // project root, one level up from docs/
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("make lint failed (exit non-zero) — golangci-lint found issues:\n%s", string(out))
		}
	})
}
