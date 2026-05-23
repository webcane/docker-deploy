---
quick_id: 260523-ins
status: complete
date: 2026-05-23
---

# Quick Task 260523-ins: Fix install.sh /main/ → /master/

## Root cause

The remote repository only has a `master` branch. All install URLs used `/main/`, causing
GitHub to return a 404 body ("404: Not Found", 14 bytes), which sh tried to execute as a
command: `sh: line 1: 404:: command not found`.

## Changes

- `install.sh:3` — usage comment URL
- `README.md:30,36` — Option 2 install block (both curl variants)
- `PREREQUISITES.md:98` — WSL2/Git Bash install example

## Verification

```
install.sh:3:# Usage: curl .../webcane/docker-deploy/master/install.sh | sh
README.md:30:curl .../webcane/docker-deploy/master/install.sh | sh
README.md:36:curl .../webcane/docker-deploy/master/install.sh | INSTALL_VERSION=v1.0.0 sh
PREREQUISITES.md:98:curl .../webcane/docker-deploy/master/install.sh | sh
```

All four URL occurrences updated. No other source files affected.
