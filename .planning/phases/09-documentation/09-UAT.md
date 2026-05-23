---
status: diagnosed
phase: 09-documentation
source: [09-01-SUMMARY.md, 09-02-SUMMARY.md, 09-03-SUMMARY.md, 09-04-SUMMARY.md]
started: 2026-05-23T00:00:00Z
updated: 2026-05-23T12:00:00Z
---

## Current Test

[testing complete]

## Tests

### 1. README.md Structure and Badges
expected: Open README.md in the repo root. You should see: a 6-badge header row (CI, Latest Release, Go Report Card, License, Open Issues, Codecov); a one-paragraph "What is docker-deploy?" description; an Installation section with 4 sub-sections (Homebrew, Install script, Manual binary, go install); a Usage section with 3 annotated scenarios; a "Learn More" section linking to PREREQUISITES.md, DEPLOY_CONFIG.md, TROUBLESHOOTING.md, COMPARISON.md; a Feedback section with a GitHub Issues link; a TON donation badge at the very bottom.
result: pass

### 2. README.md Install Commands Accuracy
expected: In README.md, verify each of the 4 install methods has exact, copy-paste-ready commands. Homebrew: `brew tap webcane/docker-deploy && brew install docker-deploy` plus the symlink command. Install script: `curl https://raw.githubusercontent.com/webcane/docker-deploy/master/install.sh | sh` with a version-pinning variant. Manual: downloads from GitHub Releases with tar/mv/chmod steps. go install: uses `GOBIN=~/.docker/cli-plugins go install ...@latest` (GOBIN note is present and explains why it's required).
result: issue
reported: "there is issue only with manual setup — Apple could not verify 'docker-deploy' is free of malware that may harm your Mac or compromise your privacy."
severity: major

### 3. README.md Usage Scenarios
expected: The Usage section has 3 scenarios each with an annotated `docker deploy` command AND a matching deploy.yaml example. Scenario 3 (config-driven) should show all 8 config fields (host, user, project_dir, compose_file, target_dir, exclude, health_timeout, health_interval). The scenarios describe different use cases (non-root user, flags-only, full config-file).
result: pass
note: "Test expected description had incorrect field names. Actual TargetConfig fields are: host, path, exclude, force, compose_file, health_timeout, health_interval, skip_env — matching config.go exactly. README Scenario 3 is correct."

### 4. PREREQUISITES.md Content
expected: Open PREREQUISITES.md. You should see 3 sections: (1) SSH Key Setup — commands for checking for an existing key, generating with ed25519, running ssh-copy-id, and testing the connection; (2) Passwordless sudo for sshuser — useradd command, docker group membership, `visudo -f /etc/sudoers.d/sshuser` with the exact sudoers line shown, and a verification command; (3) Windows users — WSL2 and Git Bash guidance, plus `go install` as a native Windows alternative.
result: pass

### 5. TROUBLESHOOTING.md Content
expected: Open TROUBLESHOOTING.md. You should see exactly 5 H2 sections: (1) SSH authentication failure; (2) Unknown host / knownhosts prompt; (3) Target directory not writable; (4) Docker not found on remote; (5) docker compose v1 detected (EOL). Each section provides actionable remediation steps (commands you can run), not just descriptions of the problem.
result: pass

### 6. COMPARISON.md Accuracy
expected: Open COMPARISON.md. You should see a comparison table with 9 rows (docker-deploy + 8 alternatives: Terraform, Ansible, Docker remote context, Manual SSH scripts, docker-compose + Watchtower, Portainer, Kamal, Full CI/CD) and 9 dimension columns. Below the table: a "When to use docker-deploy" section (5 bullets) and a "When NOT to use docker-deploy" section (5 bullets). The docker-deploy row should honestly represent its strengths and limitations.
result: pass

### 7. DEPLOY_CONFIG.md Field Reference
expected: Open DEPLOY_CONFIG.md. You should see: a full deploy.yaml schema example, a field reference table showing all 8 TargetConfig fields with their types, defaults, and descriptions. Key defaults to verify against the actual binary: health_timeout=60, health_interval=5. A "Built-in default excludes" section listing the patterns that are excluded by default (including .env, .git/, *.log, etc.). A "Config precedence" section explaining CLI flags > deploy.yaml > defaults.
result: pass

### 8. install.sh POSIX Syntax Check
expected: Run `sh -n install.sh` in the repo root. The command should complete with no errors and no output (silent success means the POSIX sh syntax is valid). Also confirm the first line is `#!/bin/sh` (not `#!/bin/bash`).
result: pass

### 9. install.sh Key Behaviors Review
expected: Open install.sh and spot-check these behaviors: (1) OS/arch detection using `uname -s` and `uname -m`; (2) INSTALL_VERSION defaults to latest from GitHub API, overridable via env var; (3) SHA256 check that hard-aborts with an error message on mismatch; (4) cosign check that prints "cosign not found — skipping signature verification, checking SHA256 only" when cosign is absent (does NOT abort); (5) binary installed to `~/.docker/cli-plugins/docker-deploy`.
result: pass
note: "Live run confirmed: darwin/arm64 detected, SHA256 OK, cosign fallback message shown, v0.6.1 installed. 'unknown' old version is expected when prior binary has no detectable version."

### 10. GoReleaser darwin Cross-Compilation Config
expected: Open .goreleaser.yaml and verify: (1) `goos` list includes both `linux` and `darwin`; (2) `goarch` list includes `amd64` and `arm64`; (3) a `signs` block is present that uses `cosign` to sign `checksums.txt`; (4) a `brews` block is present targeting `webcane/homebrew-docker-deploy` repository; (5) the Homebrew formula test uses `docker-cli-plugin-metadata` (hermetic — no Docker daemon needed).
result: pass

### 11. Release Workflow OIDC and Token Config
expected: Open .github/workflows/release.yml. In the release job, verify: (1) `id-token: write` permission is present (required for cosign OIDC signing); (2) the goreleaser-action step has `HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}` in its env block.
result: pass

### 12. Homebrew Tap Formula Live Install (SC-09-3)
expected: [Requires: tap repo created at github.com/webcane/homebrew-docker-deploy AND HOMEBREW_TAP_TOKEN secret set AND a v* tag pushed to trigger release] On a macOS or Linux machine with Homebrew installed: `brew tap webcane/docker-deploy && brew install docker-deploy` installs the binary. After `ln -sf $(brew --prefix)/bin/docker-deploy ~/.docker/cli-plugins/docker-deploy`, running `docker deploy --help` shows the plugin help output.
result: pass

### 13. End-to-End Install Smoke Test (SC-09-4)
expected: [Requires: a live v* tag on the webcane/docker-deploy repo] All three install methods should each produce a working `docker deploy --help` after install: (1) Homebrew: `brew tap webcane/docker-deploy && brew install docker-deploy` + symlink; (2) Script: `curl https://raw.githubusercontent.com/webcane/docker-deploy/master/install.sh | sh`; (3) Manual: download archive from GitHub Releases, extract, move to ~/.docker/cli-plugins/. Each method's final test: `docker deploy --help` prints plugin help.
result: pass

## Summary

total: 13
passed: 12
issues: 1
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

- truth: "Manual binary install on macOS works without extra steps after tar/mv/chmod"
  status: failed
  reason: "User reported: Apple could not verify 'docker-deploy' is free of malware that may harm your Mac or compromise your privacy."
  severity: major
  test: 2
  root_cause: "macOS Gatekeeper quarantines unsigned binaries downloaded via browser/curl+tar. Binary is not Apple-notarized. README manual install steps omit the required xattr workaround."
  artifacts:
    - path: "README.md"
      issue: "Manual install section missing: xattr -d com.apple.quarantine ~/.docker/cli-plugins/docker-deploy"
    - path: "TROUBLESHOOTING.md"
      issue: "No Gatekeeper/quarantine troubleshooting entry"
  missing:
    - "Add xattr -d com.apple.quarantine step to README.md Option 3 manual install"
    - "Add Gatekeeper quarantine section to TROUBLESHOOTING.md"
  debug_session: ""
