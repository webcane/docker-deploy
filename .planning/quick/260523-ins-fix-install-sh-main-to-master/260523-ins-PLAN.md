---
quick_id: 260523-ins
slug: fix-install-sh-main-to-master
description: Fix install.sh curl URL — /main/ → /master/ (repo has no main branch)
date: 2026-05-23
must_haves:
  truths:
    - install.sh comment uses /master/ branch URL
    - README.md install commands use /master/ branch URL
    - PREREQUISITES.md install command uses /master/ branch URL
  artifacts:
    - install.sh
    - README.md
    - PREREQUISITES.md
---

# Quick Task 260523-ins: Fix install.sh /main/ → /master/

## Root cause

The remote repository (webcane/docker-deploy) only has a `master` branch. All install
URLs referenced `/main/` which GitHub returns as a 404 response body, which sh then
tries to execute as a command.

## Task 1 — Update install.sh comment

**File:** install.sh:3
**Action:** Change `/main/` to `/master/` in the usage comment
**Verify:** `grep "master" install.sh`
**Done:** URL comment matches actual branch name

## Task 2 — Update README.md install commands

**File:** README.md (lines 30, 36)
**Action:** Change both `/main/` to `/master/` in the Option 2 install block
**Verify:** `grep -c "master" README.md`
**Done:** Both curl examples reference master branch

## Task 3 — Update PREREQUISITES.md install command

**File:** PREREQUISITES.md (line 98)
**Action:** Change `/main/` to `/master/` in the WSL2/Git Bash example
**Verify:** `grep "master" PREREQUISITES.md`
**Done:** Install command references master branch
