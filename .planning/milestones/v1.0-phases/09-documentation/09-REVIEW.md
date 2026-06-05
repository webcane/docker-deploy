---
phase: 09-documentation
reviewed: 2026-05-23T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - .github/workflows/release.yml
  - .goreleaser.yaml
  - COMPARISON.md
  - DEPLOY_CONFIG.md
  - PREREQUISITES.md
  - README.md
  - TROUBLESHOOTING.md
  - install.sh
findings:
  critical: 3
  warning: 2
  info: 1
  total: 6
status: issues_found
---

# Phase 9: Code Review Report

**Reviewed:** 2026-05-23T00:00:00Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

This phase delivers the distribution infrastructure (GoReleaser config, GitHub Actions release workflow, install.sh) and all user-facing documentation. The documentation is clear and well-structured. However, three security-impacting defects were found: the SHA256 checksum check in `install.sh` can be silently bypassed on Linux when `grep` matches nothing; the `cosign verify-blob` invocation is missing required identity-pinning flags that cause it to fail in cosign v2; and the version-pinning example in README.md and install.sh is non-functional as documented.

---

## Critical Issues

### CR-01: SHA256 checksum silently passes on Linux when grep returns no match

**File:** `install.sh:62-68`

**Issue:** The checksum lines are filtered with `grep "${ARCHIVE_NAME}" checksums.txt` before being piped to `sha256sum -c -`. On Linux (`sha256sum`), if `grep` finds no matching line — because `checksums.txt` was tampered with to remove the archive entry, is empty, or contains an unexpected filename — `grep` exits 1 but its exit code is swallowed by the pipeline. `sha256sum -c -` receives empty input and exits **0** (success). `set -e` sees the pipeline exit code of the last command (0) and the script continues without having verified anything.

Confirmed with: `echo -n "" | sha256sum -c -` exits 0 on Linux.

Note: `shasum` on macOS exits 1 on empty input, so the bypass only affects the Linux (`sha256sum`) path. Most CI and server users run Linux.

**Fix:** Check that `grep` produced output before piping, and use an anchored match to prevent partial-name collisions:

```sh
CHECKSUM_LINE=$(grep -F "  ${ARCHIVE_NAME}" "${TMPDIR}/checksums.txt" 2>/dev/null)
if [ -z "${CHECKSUM_LINE}" ]; then
  echo "ERROR: no checksum entry found for ${ARCHIVE_NAME} in checksums.txt" >&2
  exit 1
fi
echo "${CHECKSUM_LINE}" | sha256sum -c - || {
  echo "ERROR: SHA256 checksum mismatch — aborting" >&2
  exit 1
}
```

Apply the same pattern to the `shasum` branch. Using `-F` (fixed string) also prevents `.` in the filename matching unintended lines.

---

### CR-02: cosign verify-blob missing required identity flags — breaks for all users with cosign installed

**File:** `install.sh:83-89`

**Issue:** The `cosign verify-blob` invocation specifies only `--certificate`, `--signature`, and the blob path. It omits the two flags that cosign v2 requires for keyless (OIDC) verification:

- `--certificate-identity` (or `--certificate-identity-regexp`)
- `--certificate-oidc-issuer`

In cosign v2+ these flags are mandatory when using a keyless certificate. Without them, cosign exits with a non-zero code ("Error: ... certificate-identity or certificate-identity-regexp must be set"). Because `install.sh` treats any cosign failure as fatal (`exit 1` on line 88), every user who has cosign installed will get a hard failure and cannot install the binary. The intent — "optional but stronger verification when cosign is available" — is completely inverted: cosign presence breaks installation.

The `.goreleaser.yaml` signs using `COSIGN_EXPERIMENTAL=1` (GitHub Actions OIDC), which produces a Fulcio certificate with the workflow identity embedded. That identity must be explicitly pinned on the verifier side.

**Fix:**

```sh
cosign verify-blob \
  --certificate             "${TMPDIR}/checksums.txt.pem" \
  --signature               "${TMPDIR}/checksums.txt.sig" \
  --certificate-identity    "https://github.com/webcane/docker-deploy/.github/workflows/release.yml@refs/tags/v*" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  "${TMPDIR}/checksums.txt" || {
    echo "WARNING: cosign signature verification failed — binary may be tampered" >&2
    exit 1
  }
```

Use `--certificate-identity-regexp` if the exact workflow ref pattern needs flexibility across tag forms.

---

### CR-03: Version-pinning example is non-functional — INSTALL_VERSION not seen by sh

**File:** `README.md:36`, `install.sh:4`

**Issue:** Both the README and the install.sh header comment document this pattern for pinning a version:

```sh
INSTALL_VERSION=v1.0.0 curl https://raw.githubusercontent.com/webcane/docker-deploy/main/install.sh | sh
```

In a POSIX shell pipeline (`cmd1 | cmd2`), `cmd1` and `cmd2` run as sibling processes. A prefix environment assignment (`VAR=val cmd1`) sets the variable only in `cmd1`'s environment — not in `cmd2`'s. The `sh` process that executes the downloaded script does **not** inherit `INSTALL_VERSION` from the `curl` prefix. The script will see `INSTALL_VERSION` as empty, proceed to fetch the latest version from the GitHub API, and install the latest release regardless of what the user specified.

This means a user who believes they are pinning `v1.0.0` silently receives the latest release instead.

**Fix:** Update both locations to a working pattern:

```sh
# README.md — replace line 36:
curl https://raw.githubusercontent.com/webcane/docker-deploy/main/install.sh | INSTALL_VERSION=v1.0.0 sh

# install.sh comment — replace line 4:
# Version pinning: curl ... | INSTALL_VERSION=vX.Y.Z sh
```

Placing the assignment before `sh` (not before `curl`) correctly sets the variable in the sh process that executes the script.

---

## Warnings

### WR-01: cosign verify-blob failure message says WARNING but exits 1 — contradicts the label

**File:** `install.sh:87-88`

**Issue:** The error path prints `WARNING: cosign signature verification failed` but immediately calls `exit 1`. A warning implies the script will continue; here the script terminates. This is the correct security behaviour, but the misleading label will cause user confusion ("I saw a warning — why did my install abort?").

**Fix:**

```sh
echo "ERROR: cosign signature verification failed — aborting install" >&2
exit 1
```

---

### WR-02: COSIGN_EXPERIMENTAL=1 is deprecated in cosign v2 and logs a warning during signing

**File:** `.goreleaser.yaml:28`

**Issue:** `COSIGN_EXPERIMENTAL=1` was the flag to enable keyless (OIDC) signing in cosign v1. In cosign v2 the experimental mode is the default and the environment variable triggers a deprecation warning during signing: `WARN COSIGN_EXPERIMENTAL is deprecated and will be removed`. While signing continues to work, the deprecation warning clutters release logs and signals that the config is lagging behind the current cosign API.

**Fix:** Remove `COSIGN_EXPERIMENTAL=1` from the `signs[].env` block. Keyless OIDC signing works without it in cosign v2:

```yaml
signs:
  - cmd: cosign
    certificate: "${artifact}.pem"
    args:
      - sign-blob
      - "--output-certificate=${certificate}"
      - "--output-signature=${signature}"
      - "${artifact}"
      - "--yes"
    artifacts: checksum
    output: true
```

---

## Info

### IN-01: Archive name template omits version — identical filenames across all releases

**File:** `.goreleaser.yaml:19`

**Issue:** The archive `name_template` is `"{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"`, producing e.g. `docker-deploy_linux_amd64.tar.gz` — the same filename for every release. The standard GoReleaser template includes `{{ .Version }}`. While the version is present in the GitHub Releases download URL path (`/releases/download/v1.2.3/…`), archives downloaded out-of-band are indistinguishable by filename. This is a minor usability issue; it does not affect install.sh correctness since the version is in the URL.

**Fix (optional):** Add `.Version` to the template to follow standard convention and make archives self-identifying:

```yaml
archives:
  - formats: ["tar.gz"]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
```

Update `install.sh:45` to match:

```sh
ARCHIVE_NAME="${BINARY_NAME}_${INSTALL_VERSION}_${OS}_${ARCH}.tar.gz"
```

---

_Reviewed: 2026-05-23T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
