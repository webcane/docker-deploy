#!/bin/sh
# install-completions.sh — Download and install docker-deploy shell completions
# for non-Homebrew users (D-08, phase 10).
#
# Usage:
#   sh install-completions.sh
#   INSTALL_VERSION=v1.2.3 sh install-completions.sh
#
# Supported shells: bash, zsh (auto-detected from $SHELL).
# Homebrew users do not need this script — completions are installed automatically.

set -eu

REPO="webcane/docker-deploy"
# Allow callers to pin a specific version. Defaults to the latest release tag.
# When VERSION is "latest" the script fetches from the default branch (main).
VERSION="${INSTALL_VERSION:-latest}"

# ---------------------------------------------------------------------------
# Detect shell
# ---------------------------------------------------------------------------
shell=$(basename "${SHELL:-}")

case "$shell" in
  zsh)
    COMPLETION_FILE="_docker-deploy"
    ;;
  bash)
    COMPLETION_FILE="docker-deploy.bash"
    ;;
  *)
    echo "unsupported shell: $shell (supported: bash, zsh)" >&2
    exit 1
    ;;
esac

# ---------------------------------------------------------------------------
# Resolve download URL
# ---------------------------------------------------------------------------
# Use the raw GitHub content URL so we do not need to extract from a tarball.
# When VERSION is "latest" we fall back to "main" as the Git ref. Pinned
# versions (e.g. "v1.2.3") use the tag directly.
if [ "$VERSION" = "latest" ]; then
  GIT_REF="main"
else
  GIT_REF="$VERSION"
fi

DOWNLOAD_URL="https://raw.githubusercontent.com/${REPO}/${GIT_REF}/contrib/${COMPLETION_FILE}"

# ---------------------------------------------------------------------------
# Determine install destination
# ---------------------------------------------------------------------------
case "$shell" in
  zsh)
    if [ -d "/opt/homebrew/share/zsh/site-functions" ]; then
      # Homebrew Apple Silicon
      INSTALL_DIR="/opt/homebrew/share/zsh/site-functions"
    elif [ -d "/usr/local/share/zsh/site-functions" ]; then
      # Homebrew Intel
      INSTALL_DIR="/usr/local/share/zsh/site-functions"
    else
      # Non-Homebrew fallback
      INSTALL_DIR="${HOME}/.zsh/completions"
      mkdir -p "$INSTALL_DIR"
    fi
    ;;
  bash)
    if [ -d "/opt/homebrew/share/bash-completion/completions" ]; then
      # Homebrew Apple Silicon
      INSTALL_DIR="/opt/homebrew/share/bash-completion/completions"
    elif [ -d "/usr/local/share/bash-completion/completions" ]; then
      # Homebrew Intel
      INSTALL_DIR="/usr/local/share/bash-completion/completions"
    else
      # Non-Homebrew fallback
      INSTALL_DIR="${HOME}/.bash_completion.d"
      mkdir -p "$INSTALL_DIR"
    fi
    ;;
esac

DEST="${INSTALL_DIR}/${COMPLETION_FILE}"

# ---------------------------------------------------------------------------
# Download and install
# ---------------------------------------------------------------------------
echo "Downloading ${COMPLETION_FILE} from ${DOWNLOAD_URL} ..."
curl -fsSL "$DOWNLOAD_URL" -o "$DEST"

echo "Installed to: ${DEST}"

# ---------------------------------------------------------------------------
# Post-install setup hints
# ---------------------------------------------------------------------------
case "$shell" in
  zsh)
    if [ "$INSTALL_DIR" = "${HOME}/.zsh/completions" ]; then
      cat <<'MSG'

Action required — add the following to ~/.zshrc so zsh finds the completion:

  fpath=(~/.zsh/completions $fpath)
  autoload -Uz compinit && compinit

MSG
    fi
    ;;
  bash)
    if [ "$INSTALL_DIR" = "${HOME}/.bash_completion.d" ]; then
      cat <<MSG

Action required — add the following to ~/.bashrc to source the completion:

  [ -f "${HOME}/.bash_completion.d/docker-deploy.bash" ] && \\
    source "${HOME}/.bash_completion.d/docker-deploy.bash"

MSG
    fi
    ;;
esac
