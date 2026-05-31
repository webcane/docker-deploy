# Installation

Choose the method that best fits your environment.

## Install script

Pin to a release tag (recommended — avoids fetching from an unpinned `master` branch):

```bash
curl -fsSL https://raw.githubusercontent.com/webcane/docker-deploy/v0.12.2/install.sh | sh
```

Or to select a specific version at install time:

```bash
curl -fsSL https://raw.githubusercontent.com/webcane/docker-deploy/master/install.sh | INSTALL_VERSION=v0.12.2 sh
```

## Homebrew

macOS / Linux

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
