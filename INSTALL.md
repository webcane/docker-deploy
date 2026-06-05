# Installation

Choose the method that best fits your environment.

## Install script

Pin to a release tag (recommended — avoids fetching from an unpinned `master` branch):

```bash
curl -fsSL https://raw.githubusercontent.com/webcane/docker-deploy/v1.0.1/install.sh | sh
```

Or to select a specific version at install time:

```bash
curl -fsSL https://raw.githubusercontent.com/webcane/docker-deploy/master/install.sh | INSTALL_VERSION=v1.0.1 sh
```

## Homebrew

macOS

```bash
brew tap webcane/docker-deploy
brew install docker-deploy
```

### Apple Silicon note

Docker does not search `/opt/homebrew/lib/docker/cli-plugins` by default. After installing, make the plugin visible via one of:

**Option 1 — symlink into your CLI plugins directory:**

```bash
mkdir -p ~/.docker/cli-plugins
ln -sf /opt/homebrew/opt/docker-deploy/bin/docker-deploy ~/.docker/cli-plugins/docker-deploy
```

**Option 2 — add Homebrew's plugin directory to `~/.docker/config.json`:**

```json
"cliPluginsExtraDirs": ["/opt/homebrew/lib/docker/cli-plugins"]
```

## Manual binary

1. Go to the [releases page](https://github.com/webcane/docker-deploy/releases) and download the archive for your OS and architecture (e.g. `docker-deploy_linux_amd64.tar.gz`).
2. Extract the archive, move the binary to `~/.docker/cli-plugins/`, and make it executable:

```bash
tar -xzf docker-deploy_linux_amd64.tar.gz
mkdir -p ~/.docker/cli-plugins
mv docker-deploy ~/.docker/cli-plugins/docker-deploy
chmod +x ~/.docker/cli-plugins/docker-deploy
```

Replace `linux_amd64` with your OS and architecture (e.g. `darwin_arm64` for Apple Silicon).

**macOS only:** macOS Gatekeeper will block the binary because it is not Apple-notarized. Remove the quarantine attribute after install:

```bash
xattr -d com.apple.quarantine ~/.docker/cli-plugins/docker-deploy
```

## go install

```bash
GOBIN=$HOME/.docker/cli-plugins go install github.com/webcane/docker-deploy/cmd/docker-deploy@latest
```

`GOBIN` must be set to `$HOME/.docker/cli-plugins` — Docker CLI plugins must live in that directory, not in the standard `$GOPATH/bin`. Without `GOBIN`, `docker deploy` will not be discoverable by the Docker CLI.

## Shell Completions

Shell completions are provided for **bash** and **zsh**. They complete flag names (e.g. `--host`, `--path`, `--compose-file`) and subcommand names for the `docker deploy` command.

### Homebrew (automatic)

If you installed via Homebrew, completions are set up automatically — no extra steps needed.

```bash
brew tap webcane/docker-deploy
brew install docker-deploy
```

**zsh:** The completion file is installed to Homebrew's `share/zsh/site-functions/` directory, which is already on `$FPATH` on Homebrew-managed macOS. Restart your shell or run `compinit` to activate:

```bash
autoload -Uz compinit && compinit
```

**bash:** The completion file is installed to `share/bash-completion/completions/`. This directory is loaded automatically by the `bash-completion` package. If you do not have it installed yet:

```bash
brew install bash-completion@2
```

After installing `bash-completion@2`, add the following to your `~/.bashrc` or `~/.bash_profile` if it is not already present (check your existing profile — Homebrew prints the snippet during install):

```bash
[[ -r "$(brew --prefix)/etc/profile.d/bash_completion.sh" ]] && . "$(brew --prefix)/etc/profile.d/bash_completion.sh"
```

### Manual install (non-Homebrew)

For non-Homebrew installs, use the `contrib/install-completions.sh` script from the release. The script auto-detects your shell (bash or zsh) and installs the matching completion file.

Pinned to a release tag (recommended):

```bash
curl -fsSL https://raw.githubusercontent.com/webcane/docker-deploy/v1.0.1/contrib/install-completions.sh | sh
```

Or from the latest `master` (less stable):

```bash
curl -fsSL https://raw.githubusercontent.com/webcane/docker-deploy/master/contrib/install-completions.sh | sh
```

The script installs to the standard system completion directories when it can. If system directories are not writable, it falls back to user-level directories and prints the one-line snippet you need to add to your shell profile:

- **zsh fallback:** `~/.zsh/completions/` — add to `~/.zshrc`:
  ```bash
  fpath=(~/.zsh/completions $fpath)
  autoload -Uz compinit && compinit
  ```

- **bash fallback:** `~/.bash_completion.d/` — add to `~/.bashrc`:
  ```bash
  source ~/.bash_completion.d/docker-deploy.bash
  ```

### Verify

After installation, open a new shell and confirm completions are active:

```bash
docker deploy --<TAB>
```

You should see flag names listed, for example `--compose-file`, `--host`, `--path`, and others.
