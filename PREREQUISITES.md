# Prerequisites

docker-deploy connects to your VPS over SSH. You need an SSH key pair and a remote user configured with appropriate permissions.

## 1. SSH Key Setup

**Step 1: Check for an existing key**

```bash
ls ~/.ssh/id_ed25519.pub
# or
ls ~/.ssh/id_rsa.pub
```

If either file exists, you already have a key pair — skip to Step 3.

**Step 2: Generate a new key (if needed)**

```bash
ssh-keygen -t ed25519 -C "your-email@example.com"
```

Accept the default path (`~/.ssh/id_ed25519`). Setting a passphrase is recommended — it protects your key if your local machine is compromised.

**Step 3: Copy your public key to the VPS**

```bash
ssh-copy-id sshuser@vps.example.com
```

If `ssh-copy-id` is not available, append the key manually:

```bash
cat ~/.ssh/id_ed25519.pub | ssh sshuser@vps.example.com "mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys"
```

**Step 4: Test the connection**

```bash
ssh sshuser@vps.example.com echo "OK"
```

This must succeed (no password prompt) before docker-deploy will work.

> **Note:** docker-deploy uses your SSH agent or `~/.ssh/config` automatically — no flag needed once the key is in place. If you use a non-default key path or a passphrase, add the key to your agent first: `ssh-add ~/.ssh/id_ed25519`.

---

## 2. Passwordless sudo for sshuser

docker-deploy may need `sudo` to create `/opt/<project>` if it does not exist. Passwordless sudo for specific commands avoids an interactive password prompt during automated deploys.

**Step 1: SSH to the VPS as root**

```bash
ssh root@vps.example.com
```

**Step 2: Create the deploy user (if it does not exist)**

```bash
useradd -m -s /bin/bash sshuser
usermod -aG docker sshuser
```

**Step 3: Edit sudoers safely using visudo**

```bash
visudo -f /etc/sudoers.d/sshuser
```

**Step 4: Add the following line**

```
sshuser ALL=(ALL) NOPASSWD: /bin/mkdir, /bin/chown, /bin/mv, /bin/rm
```

Save and exit. `visudo` validates the syntax before writing — if there is an error it will prompt you to fix it.

**Step 5: Test from your local machine**

```bash
ssh sshuser@vps.example.com sudo mkdir -p /opt/test && ssh sshuser@vps.example.com sudo rm -rf /opt/test
```

This must complete without a password prompt.

> **Note:** Passwordless sudo is optional but recommended for CI and automated deploys. If not configured, docker-deploy will fall back to prompting for a sudo password interactively, which blocks non-interactive pipelines.

---

## 3. Windows users

docker-deploy's `install.sh` requires bash (or POSIX sh). On Windows, use **WSL2** or **Git Bash** to run the install script.

```bash
# In WSL2 or Git Bash:
curl https://raw.githubusercontent.com/webcane/docker-deploy/main/install.sh | sh
```

The binary itself builds natively for Windows. If you prefer, install via `go install` directly in a Windows terminal (no WSL2 needed):

```powershell
$env:GOBIN = "$env:USERPROFILE\.docker\cli-plugins"
go install github.com/webcane/docker-deploy/cmd/docker-deploy@latest
```
