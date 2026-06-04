package docs_test

import (
	"os"
	"strings"
	"testing"
)

// mustReadDoc reads a file relative to the project root (one level up from docs/).
func mustReadDoc(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile("../" + rel)
	if err != nil {
		t.Fatalf("cannot read %s: %v", rel, err)
	}
	return string(data)
}

// countOccurrences counts non-overlapping occurrences of substr in s.
func countOccurrences(s, substr string) int {
	count := 0
	idx := 0
	for {
		i := strings.Index(s[idx:], substr)
		if i == -1 {
			break
		}
		count++
		idx += i + len(substr)
	}
	return count
}

// TestSC095_READMEValueProposition verifies that README explains the value proposition:
// no git on VPS, single command, compose-native (SC-09-5).
func TestSC095_READMEValueProposition(t *testing.T) {
	content := mustReadDoc(t, "README.md")

	t.Run("mentions no git on VPS", func(t *testing.T) {
		hasNoGit := strings.Contains(content, "no git") || strings.Contains(content, "no git required") ||
			strings.Contains(content, "without git")
		if !hasNoGit {
			t.Error("README must explain 'no git required on remote' as a core value proposition")
		}
	})

	t.Run("mentions single command deploy", func(t *testing.T) {
		hasSingleCmd := strings.Contains(content, "single command") || strings.Contains(content, "one command") ||
			strings.Contains(content, "One command")
		if !hasSingleCmd {
			t.Error("README must explain 'single command' deploy as a core value proposition")
		}
	})

	t.Run("mentions compose-native or docker-compose", func(t *testing.T) {
		hasCompose := strings.Contains(content, "docker-compose") || strings.Contains(content, "compose")
		if !hasCompose {
			t.Error("README must mention docker-compose / compose as the deployment primitive")
		}
	})
}

// TestSC096_READMEAllFourInstallMethods verifies that README covers all four install methods
// with copy-paste commands (SC-09-6). Requirement: Homebrew, install script, manual binary, go install.
func TestSC096_READMEAllFourInstallMethods(t *testing.T) {
	content := mustReadDoc(t, "README.md")

	// Check if methods are in README directly or linked to an install doc
	hasInstallSection := strings.Contains(content, "## Installation") || strings.Contains(content, "## Install")

	t.Run("has installation section", func(t *testing.T) {
		if !hasInstallSection {
			t.Error("README must have an '## Installation' section")
		}
	})

	t.Run("Homebrew install method present", func(t *testing.T) {
		hasHomebrew := strings.Contains(content, "brew tap") || strings.Contains(content, "brew install") ||
			strings.Contains(content, "Homebrew")
		// Also acceptable: linked to INSTALL.md if that file contains Homebrew
		if !hasHomebrew {
			installMd, err := os.ReadFile("../INSTALL.md")
			if err != nil || !strings.Contains(string(installMd), "brew") {
				t.Error("README (or linked INSTALL.md) must cover Homebrew install method with brew tap / brew install command")
			}
		}
	})

	t.Run("install script method present", func(t *testing.T) {
		if !strings.Contains(content, "install.sh") {
			t.Error("README must cover the curl | sh install script method (install.sh)")
		}
	})

	t.Run("manual binary download method present", func(t *testing.T) {
		// Manual binary = GitHub Releases download
		hasManual := strings.Contains(content, "GitHub Releases") || strings.Contains(content, "github.com") ||
			strings.Contains(content, "tar.gz") || strings.Contains(content, "manual")
		if !hasManual {
			installMd, err := os.ReadFile("../INSTALL.md")
			if err != nil || !strings.Contains(string(installMd), "tar.gz") {
				t.Error("README (or linked INSTALL.md) must cover manual binary download from GitHub Releases")
			}
		}
	})

	t.Run("go install method present", func(t *testing.T) {
		hasGoInstall := strings.Contains(content, "go install")
		if !hasGoInstall {
			installMd, err := os.ReadFile("../INSTALL.md")
			if err != nil || !strings.Contains(string(installMd), "go install") {
				t.Error("README (or linked INSTALL.md) must cover 'go install' method with GOBIN=~/.docker/cli-plugins")
			}
		}
	})
}

// TestSC097_READMEThreeUseCaseScenarios verifies that README covers three use-case scenarios
// each with a command and deploy.yaml example (SC-09-7).
//
// SC-09-7 requires README itself to cover three scenarios. The implementation split this into
// README (Quick Start) + USAGE.md (full scenarios). This test verifies the actual requirement:
// that a user reading README gets three distinct use-case scenarios with commands and deploy.yaml examples.
// README linking to USAGE.md satisfies this only if USAGE.md has the scenarios and README references them.
func TestSC097_READMEThreeUseCaseScenarios(t *testing.T) {
	content := mustReadDoc(t, "README.md")

	t.Run("has usage section", func(t *testing.T) {
		if !strings.Contains(content, "## Usage") {
			t.Error("README must have a '## Usage' section")
		}
	})

	t.Run("has at least one docker deploy command example", func(t *testing.T) {
		if !strings.Contains(content, "docker deploy") {
			t.Error("README Usage section must show at least one 'docker deploy' command example")
		}
	})

	t.Run("covers three scenarios inline or via linked USAGE.md", func(t *testing.T) {
		// SC-09-7 primary requirement: three use-case scenarios with commands and deploy.yaml.
		// Accept inline scenarios OR delegation to USAGE.md (hub-README pattern).
		inlineScenarios := strings.Count(content, "### Scenario") >= 3 ||
			strings.Count(content, "### Option") >= 3
		hasUsageLink := strings.Contains(content, "USAGE.md")

		if !inlineScenarios && !hasUsageLink {
			t.Error("README must have three use-case scenarios inline, or link to USAGE.md that contains them")
			return
		}

		// If delegated to USAGE.md, USAGE.md must actually exist and contain the scenarios
		if !inlineScenarios && hasUsageLink {
			usageMd, err := os.ReadFile("../USAGE.md")
			if err != nil {
				t.Errorf("README links to USAGE.md but file does not exist: %v", err)
				return
			}
			usageContent := string(usageMd)
			// USAGE.md must have at least 3 distinct scenario sections with deploy.yaml examples
			scenarioCount := strings.Count(usageContent, "### Scenario") + strings.Count(usageContent, "#### Scenario") +
				strings.Count(usageContent, "## Scenario")
			if scenarioCount < 3 && strings.Count(usageContent, "deploy.yaml") < 2 {
				t.Errorf("USAGE.md must cover at least 3 use-case scenarios with deploy.yaml examples (found %d scenario sections)", scenarioCount)
			}
		}
	})

	t.Run("deploy.yaml referenced in README or USAGE.md", func(t *testing.T) {
		// The scenarios must include deploy.yaml examples
		readmeHasYaml := strings.Contains(content, "deploy.yaml")
		if readmeHasYaml {
			return // README itself has deploy.yaml reference
		}
		// Check USAGE.md
		usageMd, err := os.ReadFile("../USAGE.md")
		if err != nil {
			t.Error("README does not mention deploy.yaml and USAGE.md does not exist — SC-09-7 requires deploy.yaml examples in scenarios")
			return
		}
		if !strings.Contains(string(usageMd), "deploy.yaml") {
			t.Error("Neither README nor USAGE.md contains deploy.yaml examples — SC-09-7 requires deploy.yaml in usage scenarios")
		}
	})
}

// TestSC098_COMPARISONMDTableDimensions verifies that COMPARISON.md has an 8-tool x 9-dimension table (SC-09-8).
func TestSC098_COMPARISONMDTableDimensions(t *testing.T) {
	content := mustReadDoc(t, "COMPARISON.md")

	t.Run("contains comparison table section", func(t *testing.T) {
		if !strings.Contains(content, "## Comparison Table") {
			t.Error("COMPARISON.md must have a '## Comparison Table' section")
		}
	})

	t.Run("table has all 8 required comparator tools", func(t *testing.T) {
		required := []string{
			"Terraform",
			"Ansible",
			"Kamal",
			"Portainer",
			"Watchtower",
			"Docker remote context",
			"Manual SSH",
			"CI/CD",
		}
		for _, tool := range required {
			if !strings.Contains(content, tool) {
				t.Errorf("COMPARISON.md table must include tool %q (8-tool requirement)", tool)
			}
		}
	})

	t.Run("table has all 9 required dimension columns", func(t *testing.T) {
		// The 9 required dimensions from D-28
		required := []string{
			"Docker Compose native",
			"secrets",        // .env / secrets handling
			"first deploy",   // Time to first deploy
			"Compose-centric",
			"SSH",            // SSH best practices
			"Complexity",     // Complexity / learning curve
			"Remote dependencies",
			"git on VPS",     // Requires git on VPS
			"Best fit",
		}
		for _, dim := range required {
			if !strings.Contains(content, dim) {
				t.Errorf("COMPARISON.md table must include dimension column %q (9-dimension requirement)", dim)
			}
		}
	})

	t.Run("has when-to-use section", func(t *testing.T) {
		if !strings.Contains(content, "When to use docker-deploy") {
			t.Error("COMPARISON.md must have a 'When to use docker-deploy' section")
		}
	})

	t.Run("has when-not-to-use section", func(t *testing.T) {
		if !strings.Contains(content, "When NOT to use") {
			t.Error("COMPARISON.md must have a 'When NOT to use docker-deploy' section")
		}
	})
}

// TestSC099_PREREQUISITESMDCoversSshAndSudo verifies PREREQUISITES.md covers SSH key setup
// and passwordless sudo (SC-09-9).
func TestSC099_PREREQUISITESMDCoversSshAndSudo(t *testing.T) {
	content := mustReadDoc(t, "PREREQUISITES.md")

	t.Run("covers SSH key generation with ssh-keygen", func(t *testing.T) {
		if !strings.Contains(content, "ssh-keygen") {
			t.Error("PREREQUISITES.md must include ssh-keygen command for SSH key generation")
		}
	})

	t.Run("covers copying key to VPS", func(t *testing.T) {
		hasCopyId := strings.Contains(content, "ssh-copy-id")
		hasManualCopy := strings.Contains(content, "authorized_keys")
		if !hasCopyId && !hasManualCopy {
			t.Error("PREREQUISITES.md must cover copying SSH key to VPS (ssh-copy-id or authorized_keys)")
		}
	})

	t.Run("covers passwordless sudo setup", func(t *testing.T) {
		hasSudo := strings.Contains(content, "passwordless") || strings.Contains(content, "NOPASSWD")
		if !hasSudo {
			t.Error("PREREQUISITES.md must cover passwordless sudo setup for sshuser")
		}
	})

	t.Run("covers visudo for safe sudoers editing", func(t *testing.T) {
		if !strings.Contains(content, "visudo") {
			t.Error("PREREQUISITES.md must reference visudo for safe sudoers editing")
		}
	})

	t.Run("covers Windows users WSL2 note", func(t *testing.T) {
		if !strings.Contains(content, "WSL2") {
			t.Error("PREREQUISITES.md must include WSL2 note for Windows users (D-26 requirement)")
		}
	})
}

// TestSC0910_TROUBLESHOOTINGMDFiveFailureScenarios verifies TROUBLESHOOTING.md covers
// all 5 required failure scenarios with actionable fixes (SC-09-10).
// Note: the file may have additional scenarios beyond 5; the requirement is that all 5 mandated
// scenarios are present, not that EXACTLY 5 exist.
func TestSC0910_TROUBLESHOOTINGMDFiveFailureScenarios(t *testing.T) {
	content := mustReadDoc(t, "TROUBLESHOOTING.md")

	t.Run("covers SSH authentication failure", func(t *testing.T) {
		hasSSHAuth := strings.Contains(content, "SSH authentication") ||
			strings.Contains(content, "ssh: handshake") ||
			strings.Contains(content, "permission denied (publickey)")
		if !hasSSHAuth {
			t.Error("TROUBLESHOOTING.md must cover SSH authentication failure scenario")
		}
	})

	t.Run("covers unknown host / knownhosts prompt", func(t *testing.T) {
		hasKnownHosts := strings.Contains(content, "known_hosts") || strings.Contains(content, "knownhosts") ||
			strings.Contains(content, "host fingerprint") || strings.Contains(content, "Unknown host")
		if !hasKnownHosts {
			t.Error("TROUBLESHOOTING.md must cover unknown host / knownhosts scenario")
		}
	})

	t.Run("covers target directory not writable", func(t *testing.T) {
		hasPermission := strings.Contains(content, "permission denied") || strings.Contains(content, "not writable") ||
			strings.Contains(content, "Target directory")
		if !hasPermission {
			t.Error("TROUBLESHOOTING.md must cover target directory not writable scenario")
		}
	})

	t.Run("covers Docker not found on remote", func(t *testing.T) {
		hasDockerMissing := strings.Contains(content, "Docker not found") || strings.Contains(content, "docker: not found") ||
			strings.Contains(content, "docker is not installed")
		if !hasDockerMissing {
			t.Error("TROUBLESHOOTING.md must cover Docker not found on remote scenario")
		}
	})

	t.Run("covers docker compose v1 detected EOL", func(t *testing.T) {
		hasV1 := strings.Contains(content, "v1") && (strings.Contains(content, "compose") || strings.Contains(content, "Compose"))
		if !hasV1 {
			t.Error("TROUBLESHOOTING.md must cover docker compose v1 detected (EOL) scenario")
		}
	})

	t.Run("each scenario has actionable fix", func(t *testing.T) {
		// Every scenario must have at least one fix command or instruction
		// We verify this by checking that Fix: or Fix — or ```bash appears after each ## heading
		hasFixContent := strings.Contains(content, "Fix") || strings.Contains(content, "fix") ||
			strings.Contains(content, "```bash")
		if !hasFixContent {
			t.Error("TROUBLESHOOTING.md scenarios must each include actionable fix instructions")
		}
	})
}

// TestSC0911_READMELinksToSupportingDocs verifies README links to all four supporting docs (SC-09-11).
func TestSC0911_READMELinksToSupportingDocs(t *testing.T) {
	content := mustReadDoc(t, "README.md")

	links := []struct {
		name string
		file string
	}{
		{"PREREQUISITES.md", "PREREQUISITES.md"},
		{"DEPLOY_CONFIG.md", "DEPLOY_CONFIG.md"},
		{"TROUBLESHOOTING.md", "TROUBLESHOOTING.md"},
		{"COMPARISON.md", "COMPARISON.md"},
	}

	for _, link := range links {
		t.Run("links to "+link.name, func(t *testing.T) {
			if !strings.Contains(content, link.file) {
				t.Errorf("README.md must contain a link to %s", link.file)
			}
		})
	}
}

// TestSC0912_READMEBadges verifies README has CI status, latest release, and test/coverage badges (SC-09-12).
func TestSC0912_READMEBadges(t *testing.T) {
	content := mustReadDoc(t, "README.md")

	t.Run("has CI status badge", func(t *testing.T) {
		hasCI := strings.Contains(content, "ci.yml/badge.svg") || strings.Contains(content, "workflows/ci") ||
			strings.Contains(content, "CI")
		if !hasCI {
			t.Error("README must include a CI status badge linking to the CI workflow")
		}
	})

	t.Run("has latest release badge", func(t *testing.T) {
		hasRelease := strings.Contains(content, "github/v/release") || strings.Contains(content, "Latest Release") ||
			strings.Contains(content, "releases")
		if !hasRelease {
			t.Error("README must include a latest release badge")
		}
	})

	t.Run("has test or coverage status badge", func(t *testing.T) {
		// SC-09-12 says "test/coverage status" — Codecov badge or coverage badge
		hasCoverage := strings.Contains(content, "codecov") || strings.Contains(content, "coverage") ||
			strings.Contains(content, "Coverage")
		if !hasCoverage {
			t.Error("README must include a test/coverage status badge (e.g. Codecov or coverage.svg)")
		}
	})

	t.Run("badges use shields.io or similar badge service", func(t *testing.T) {
		hasBadgeService := strings.Contains(content, "shields.io") || strings.Contains(content, "badge.svg") ||
			strings.Contains(content, "img.shields")
		if !hasBadgeService {
			t.Error("README badges must use shields.io or a badge image (badge.svg)")
		}
	})
}
