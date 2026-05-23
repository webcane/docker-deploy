# Troubleshooting

This guide covers the five most common failure scenarios when using docker-deploy, with symptoms and actionable fixes.

---

## 1. SSH authentication failure

**Symptom:** Deploy fails with output similar to:

```
SSH connection failed: ssh: handshake failed: ssh: unable to authenticate
permission denied (publickey)
```

**Causes and fixes:**

- **SSH key not on the VPS:** Run `ssh-copy-id sshuser@vps.example.com` or manually append your public key to `~/.ssh/authorized_keys` on the remote. See [PREREQUISITES.md](PREREQUISITES.md) for step-by-step instructions.

- **Wrong user in host URI:** Verify the user in your `--host` flag or `deploy.yaml`:
  ```bash
  docker deploy --host ssh://sshuser@vps.example.com --dry-run
  ```
  The user must match the account that has your public key.

- **SSH agent not running:** Start the agent and add your key:
  ```bash
  eval $(ssh-agent)
  ssh-add ~/.ssh/id_ed25519
  ```

- **Key passphrase not loaded:** Even if the agent is running, the key must be added:
  ```bash
  ssh-add ~/.ssh/id_ed25519
  ```

**Verify:** The following command must succeed before docker-deploy will work:
```bash
ssh sshuser@vps.example.com echo OK
```

---

## 2. Unknown host / knownhosts prompt

**Symptom:** Deploy stops with a prompt asking you to verify the host fingerprint:

```
The authenticity of host 'vps.example.com' can't be established.
ED25519 key fingerprint is SHA256:...
Are you sure you want to continue connecting (yes/no/[fingerprint])?
```

**Explanation:** docker-deploy uses strict known-hosts verification — `InsecureIgnoreHostKey` is never used. On first connection to a new host, you must confirm the fingerprint.

**Fix:** The prompt shows the server's fingerprint. If you trust the host, type `yes`. The host is added to `~/.ssh/known_hosts` automatically. Subsequent deploys are non-interactive.

**Alternative:** Run a plain SSH command first — this adds the key to `known_hosts` via standard SSH, so docker-deploy can connect without prompting:
```bash
ssh sshuser@vps.example.com echo OK
```

---

## 3. Target directory not writable

**Symptom:** Deploy fails with output similar to:

```
Deploy failed: mkdir /opt/myapp: permission denied
```

**Cause:** `/opt/<project>` does not exist and `sshuser` cannot create it without `sudo`.

**Fix — Option A (recommended):** Configure passwordless sudo as described in [PREREQUISITES.md](PREREQUISITES.md). docker-deploy will create the directory automatically on the next deploy.

**Fix — Option B:** Pre-create the directory as root:
```bash
ssh root@vps.example.com "mkdir -p /opt/myapp && chown sshuser:sshuser /opt/myapp"
```

**Fix — Option C:** Use a path the user already owns:
```bash
docker deploy --host ssh://sshuser@vps.example.com --path /home/sshuser/myapp
```
Or set `path: /home/sshuser/myapp` in `deploy.yaml`.

---

## 4. Docker not found on remote

**Symptom:** Deploy fails at the pre-flight stage with:

```
docker: not found
```
or
```
pre-flight: docker is not installed on the remote host
```

**Fix:** Install Docker on the VPS using the official install script:
```bash
ssh sshuser@vps.example.com "curl -fsSL https://get.docker.com | sh"
```

Then add the user to the `docker` group so it can run Docker without `sudo`:
```bash
ssh root@vps.example.com "usermod -aG docker sshuser"
```

Log out and back in (or run `newgrp docker`) for the group change to take effect.

**Verify:**
```bash
ssh sshuser@vps.example.com docker info
```

---

## 5. docker compose v1 detected (EOL)

**Symptom:** Pre-flight fails with:

```
docker-compose v1 detected — docker-deploy requires docker compose v2
```

**Cause:** The VPS has the legacy `docker-compose` binary (v1, Python-based) but not the `docker compose` plugin (v2, Go-based). Docker Compose v1 reached end-of-life in May 2023; docker-deploy does not support it.

**Fix:** Install the Docker Compose v2 plugin:

```bash
# Ubuntu / Debian
ssh sshuser@vps.example.com "sudo apt-get install -y docker-compose-plugin"
```

For other distributions, see the [official Docker Compose install guide](https://docs.docker.com/compose/install/).

**Verify:**
```bash
ssh sshuser@vps.example.com docker compose version
```

The output must start with `Docker Compose version v2.` (e.g. `Docker Compose version v2.24.0`).

---

## 6. macOS Gatekeeper blocks the binary (manual install)

**Symptom:** After manually downloading from GitHub Releases, macOS shows:

```
"docker-deploy" cannot be opened because Apple cannot check it for malicious software.
```
or
```
Apple could not verify "docker-deploy" is free of malware that may harm your Mac or compromise your privacy.
```

**Cause:** Binaries downloaded from the internet are quarantined by macOS Gatekeeper. docker-deploy is an open-source binary that is not Apple-notarized, so Gatekeeper blocks it on first run.

**Fix:** Remove the quarantine attribute:
```bash
xattr -d com.apple.quarantine ~/.docker/cli-plugins/docker-deploy
```

**Verify:**
```bash
docker deploy --help
```

**Note:** This issue only affects the manual binary install (Option 3). The Homebrew install and the install script (`install.sh`) do not trigger this warning because they handle the quarantine attribute automatically.
