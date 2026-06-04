package docs_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// mustRead reads a file relative to the project root (one level up from docs/).
// Tests in this package run from the docs/ directory, so ../ is the project root.
func mustReadDist(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile("../" + rel)
	if err != nil {
		t.Fatalf("cannot read %s: %v", rel, err)
	}
	return string(data)
}

// TestSC091_GoReleaserDarwinBuilds verifies that .goreleaser.yaml is configured to produce
// darwin/amd64 and darwin/arm64 binaries on tag push (SC-09-1).
func TestSC091_GoReleaserDarwinBuilds(t *testing.T) {
	content := mustReadDist(t, ".goreleaser.yaml")

	checks := []struct {
		name    string
		want    string
		purpose string
	}{
		{"darwin in goos", "darwin", "darwin must appear in the goos list to produce darwin builds"},
		{"amd64 in goarch", "amd64", "amd64 must appear in goarch to produce darwin/amd64"},
		{"arm64 in goarch", "arm64", "arm64 must appear in goarch to produce darwin/arm64"},
		{"linux also present", "linux", "linux must still be present alongside darwin"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(content, c.want) {
				t.Errorf(".goreleaser.yaml missing %q: %s", c.want, c.purpose)
			}
		})
	}
}

// TestSC092_InstallShSHA256AndCosignFallback verifies that install.sh always verifies SHA256
// against checksums.txt and prints the exact cosign fallback message when cosign is absent (SC-09-2).
func TestSC092_InstallShSHA256AndCosignFallback(t *testing.T) {
	content := mustReadDist(t, "install.sh")

	t.Run("starts with POSIX shebang", func(t *testing.T) {
		if !strings.HasPrefix(content, "#!/bin/sh") {
			t.Errorf("install.sh must start with #!/bin/sh (POSIX sh); got prefix: %q", content[:20])
		}
	})

	t.Run("references checksums.txt for SHA256 verification", func(t *testing.T) {
		if !strings.Contains(content, "checksums.txt") {
			t.Error("install.sh must reference checksums.txt for SHA256 verification")
		}
	})

	t.Run("uses sha256sum or shasum for verification", func(t *testing.T) {
		hasSha256sum := strings.Contains(content, "sha256sum")
		hasShasum := strings.Contains(content, "shasum")
		if !hasSha256sum && !hasShasum {
			t.Error("install.sh must use sha256sum or shasum for checksum verification")
		}
	})

	t.Run("aborts with error on SHA256 mismatch", func(t *testing.T) {
		// The script must hard-fail (exit 1) when checksum does not match
		if !strings.Contains(content, "SHA256 checksum mismatch") {
			t.Error("install.sh must print 'SHA256 checksum mismatch' and exit 1 when checksum fails")
		}
	})

	t.Run("cosign fallback prints exact required message", func(t *testing.T) {
		// D-14 requires this exact message when cosign is absent
		exactMsg := "cosign not found — skipping signature verification, checking SHA256 only"
		if !strings.Contains(content, exactMsg) {
			t.Errorf("install.sh must print exactly: %q when cosign is absent; not found in script", exactMsg)
		}
	})

	t.Run("does not hard-fail when cosign is absent", func(t *testing.T) {
		// The cosign-not-found branch must NOT contain exit 1 — it should continue
		lines := strings.Split(content, "\n")
		inElseBranch := false
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Find the else branch for cosign check
			if strings.Contains(line, "cosign not found") {
				inElseBranch = true
			}
			if inElseBranch && (trimmed == "fi" || (i > 0 && strings.Contains(lines[i-1], "cosign not found"))) {
				// Once we've seen the cosign-not-found message line and reached fi, stop
				if trimmed == "fi" {
					break
				}
				if trimmed == "exit 1" {
					t.Errorf("install.sh must NOT exit 1 when cosign is absent (graceful fallback required); found 'exit 1' near cosign-not-found message")
				}
			}
		}
	})

	t.Run("places binary in cli-plugins directory", func(t *testing.T) {
		if !strings.Contains(content, ".docker/cli-plugins") {
			t.Error("install.sh must place binary in ~/.docker/cli-plugins")
		}
	})

	t.Run("supports INSTALL_VERSION env var for pinning", func(t *testing.T) {
		if !strings.Contains(content, "INSTALL_VERSION") {
			t.Error("install.sh must support INSTALL_VERSION env var for version pinning")
		}
	})

	t.Run("POSIX syntax check via sh -n", func(t *testing.T) {
		cmd := exec.Command("sh", "-n", "../install.sh")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("sh -n install.sh failed (POSIX syntax error): %v\n%s", err, out)
		}
	})

	t.Run("does not contain bash shebang", func(t *testing.T) {
		if strings.Contains(content, "#!/bin/bash") {
			t.Error("install.sh must use #!/bin/sh (POSIX), not #!/bin/bash")
		}
	})

	t.Run("has set -e for fail-fast execution", func(t *testing.T) {
		if !strings.Contains(content, "set -e") {
			t.Error("install.sh must have 'set -e' to abort on any command failure")
		}
	})
}

// TestSC093_GoReleaserCosignKeylessSigning verifies that .goreleaser.yaml contains a signs block
// that uses cosign keyless signing (COSIGN_EXPERIMENTAL not required, but cosign + checksum artifact must be present) (SC-09-3).
func TestSC093_GoReleaserCosignKeylessSigning(t *testing.T) {
	content := mustReadDist(t, ".goreleaser.yaml")

	t.Run("signs block uses cosign", func(t *testing.T) {
		if !strings.Contains(content, "cosign") {
			t.Error(".goreleaser.yaml must contain a signs block using cosign")
		}
	})

	t.Run("signs block targets checksums artifact", func(t *testing.T) {
		if !strings.Contains(content, "artifacts: checksum") {
			t.Error(".goreleaser.yaml signs block must target 'artifacts: checksum' to sign checksums.txt")
		}
	})

	t.Run("signs block uses sign-blob command", func(t *testing.T) {
		if !strings.Contains(content, "sign-blob") {
			t.Error(".goreleaser.yaml signs block must use 'sign-blob' cosign subcommand")
		}
	})

	t.Run("release workflow has id-token permission for OIDC cosign", func(t *testing.T) {
		releaseYml := mustReadDist(t, ".github/workflows/release.yml")
		if !strings.Contains(releaseYml, "id-token: write") {
			t.Error("release.yml must have 'id-token: write' permission for OIDC cosign keyless signing")
		}
	})

	t.Run("release workflow has cosign-installer step", func(t *testing.T) {
		releaseYml := mustReadDist(t, ".github/workflows/release.yml")
		if !strings.Contains(releaseYml, "cosign") {
			t.Error("release.yml must install cosign (e.g. sigstore/cosign-installer) to perform keyless signing")
		}
	})

	t.Run("brews block has HOMEBREW_TAP_TOKEN wired", func(t *testing.T) {
		if !strings.Contains(content, "HOMEBREW_TAP_TOKEN") {
			t.Error(".goreleaser.yaml brews block must reference HOMEBREW_TAP_TOKEN for tap push")
		}
	})
}
