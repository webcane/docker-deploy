---
title: Distribution approach decisions
date: 2026-05-19
context: Exploration session — how docker-deploy will be distributed to end users
---

# Distribution Approach Decisions

## Context

docker-deploy is installed on the developer's local machine only (e.g. macOS).
Primary user is a solo developer. Multi-server deploy is deferred to v2/v3.
The binary is a Docker CLI plugin — it must land in `~/.docker/cli-plugins/docker-deploy`.

## Approaches — ordered by implementation simplicity

### 1. Manual binary download (already works)
- GoReleaser (wired in Phase 1) produces darwin/linux × amd64/arm64 binaries on GitHub Releases
- User downloads, `chmod +x`, moves to `~/.docker/cli-plugins/`
- Zero additional implementation work

### 2. Shell install script
- `curl -sSL https://raw.githubusercontent.com/.../install.sh | sh`
- Script detects `$(uname -s)` / `$(uname -m)`, constructs the release download URL, places binary in `~/.docker/cli-plugins/docker-deploy`, sets `+x`
- ~30 lines of shell; hosted in the repo root
- Covers CI/scripting use cases without requiring Go or Homebrew

### 3. Homebrew tap
- New repo: `homebrew-docker-deploy`
- Formula downloads the correct bottle from GitHub Releases and places the binary
- `brew install <user>/docker-deploy/docker-deploy`
- Best UX for Mac developers; GoReleaser can auto-generate the formula on release

### Deferred / not planned

| Method | Reason deferred |
|--------|----------------|
| `go install` | Binary lands in `$GOPATH/bin`, not `~/.docker/cli-plugins/` — requires manual extra step; poor UX |
| apt/deb | Significant ops overhead (hosted apt repo); Linux is secondary target |
| Docker image | Unusual for a local CLI plugin; adds no value over binary download |

## Decisions

- All three active methods (manual, script, Homebrew) are bundled into **Phase 9: Distribution & Documentation**
- Implementation order matches simplicity: GoReleaser verification → install.sh → Homebrew tap
- README install section covers all three with copy-paste commands
