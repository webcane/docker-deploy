#!/bin/sh
# install.sh — One-liner installer for docker-deploy
# Usage: curl https://raw.githubusercontent.com/webcane/docker-deploy/main/install.sh | sh
# Version pinning: curl ... | INSTALL_VERSION=v1.2.3 sh
set -e

# ── Variables ────────────────────────────────────────────────────────────────
REPO="webcane/docker-deploy"
INSTALL_DIR="${HOME}/.docker/cli-plugins"
BINARY_NAME="docker-deploy"

# Allow version pinning via env var; otherwise fetch latest from GitHub API
if [ -z "${INSTALL_VERSION:-}" ]; then
  INSTALL_VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed 's/.*"tag_name": "\(.*\)".*/\1/')
fi

if [ -z "${INSTALL_VERSION}" ]; then
  echo "ERROR: could not determine latest release version — set INSTALL_VERSION=vX.Y.Z to pin" >&2
  exit 1
fi

# ── OS / arch detection ───────────────────────────────────────────────────────
_os=$(uname -s)
case "${_os}" in
  Linux)  OS="linux"  ;;
  Darwin) OS="darwin" ;;
  *)
    echo "ERROR: unsupported operating system: ${_os}" >&2
    exit 1
    ;;
esac

_arch=$(uname -m)
case "${_arch}" in
  x86_64)          ARCH="amd64" ;;
  aarch64 | arm64) ARCH="arm64" ;;
  *)
    echo "ERROR: unsupported CPU architecture: ${_arch}" >&2
    exit 1
    ;;
esac

ARCHIVE_NAME="${BINARY_NAME}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${INSTALL_VERSION}"

echo "Installing ${BINARY_NAME} ${INSTALL_VERSION} (${OS}/${ARCH}) ..."

# ── Download ──────────────────────────────────────────────────────────────────
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL "${BASE_URL}/${ARCHIVE_NAME}"   -o "${TMPDIR}/${ARCHIVE_NAME}"
curl -fsSL "${BASE_URL}/checksums.txt"     -o "${TMPDIR}/checksums.txt"

# ── SHA256 verification (always enforced) ─────────────────────────────────────
# Move into TMPDIR so grep + sha256 can resolve the relative filename in checksums.txt
cd "${TMPDIR}"

if command -v sha256sum >/dev/null 2>&1; then
  # CR-01 fix: grep empty output silently passes on Linux — extract to file first
  grep "${ARCHIVE_NAME}" checksums.txt > "${TMPDIR}/archive.sha256" || {
    echo "ERROR: ${ARCHIVE_NAME} not found in checksums.txt — cannot verify" >&2
    exit 1
  }
  sha256sum -c "${TMPDIR}/archive.sha256" || {
    echo "ERROR: SHA256 checksum mismatch — aborting" >&2
    exit 1
  }
elif command -v shasum >/dev/null 2>&1; then
  grep "${ARCHIVE_NAME}" checksums.txt > "${TMPDIR}/archive.sha256" || {
    echo "ERROR: ${ARCHIVE_NAME} not found in checksums.txt — cannot verify" >&2
    exit 1
  }
  shasum -a 256 -c "${TMPDIR}/archive.sha256" || {
    echo "ERROR: SHA256 checksum mismatch — aborting" >&2
    exit 1
  }
else
  echo "ERROR: neither sha256sum nor shasum found — cannot verify checksum" >&2
  exit 1
fi

# Return to original directory
cd - >/dev/null

# ── cosign verification (optional; fall back gracefully) ──────────────────────
if command -v cosign >/dev/null 2>&1; then
  curl -fsSL "${BASE_URL}/checksums.txt.sig" -o "${TMPDIR}/checksums.txt.sig"
  curl -fsSL "${BASE_URL}/checksums.txt.pem" -o "${TMPDIR}/checksums.txt.pem"
  # CR-02 fix: cosign v2 requires explicit identity flags for keyless verification
  # WR-01 fix: hard failure → ERROR label, not WARNING
  cosign verify-blob \
    --certificate "${TMPDIR}/checksums.txt.pem" \
    --signature   "${TMPDIR}/checksums.txt.sig" \
    --certificate-identity-regexp "https://github.com/${REPO}/" \
    --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
    "${TMPDIR}/checksums.txt" || {
    echo "ERROR: cosign signature verification failed — binary may be tampered" >&2
    exit 1
  }
else
  echo "cosign not found — skipping signature verification, checking SHA256 only"
fi

# ── Install ───────────────────────────────────────────────────────────────────
tar -xzf "${TMPDIR}/${ARCHIVE_NAME}" -C "${TMPDIR}"
mkdir -p "${INSTALL_DIR}"

# Capture old version for upgrade message (suppress errors if not yet installed)
OLD_VERSION=""
if [ -x "${INSTALL_DIR}/${BINARY_NAME}" ]; then
  OLD_VERSION=$("${INSTALL_DIR}/${BINARY_NAME}" --version 2>/dev/null || echo "unknown")
fi

mv "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

if [ -n "${OLD_VERSION}" ]; then
  echo "Updated ${BINARY_NAME} ${OLD_VERSION} → ${INSTALL_VERSION}"
else
  echo "Installed ${BINARY_NAME} ${INSTALL_VERSION} to ${INSTALL_DIR}/${BINARY_NAME}"
fi

echo "Run: docker deploy --help"
