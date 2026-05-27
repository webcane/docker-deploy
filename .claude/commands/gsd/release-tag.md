---
name: gsd:release-tag
description: Bump semver, update README and INSTALL.md install URLs, commit, tag, and push to trigger CI release
argument-hint: "[major|minor|patch]"
allowed-tools:
  - Read
  - Edit
  - Bash
  - AskUserQuestion
---

<objective>
Cut a new release from the current branch:
1. Detect the latest git tag
2. Compute the next version (or accept one from $ARGUMENTS)
3. Update the hardcoded version in README.md and INSTALL.md
4. Commit the file changes, create the tag, push both
5. GitHub Actions fires on the `v*` tag and builds the release binary

This is the **sole entry point** for creating releases. Do not tag manually outside this skill.
</objective>

<process>

## Wave 0 — Pre-release checks

These checks run **before** any user-facing prompts or file changes. All three must pass before proceeding to Step 1.

### Wave 0 Step A — Unit tests

```bash
echo "▶ go test ./..."
make test
```

If `make test` exits non-zero: print `ABORT: unit tests failed — fix failures before releasing` and stop with no file changes.

If `make test` passes: print `PASS` and continue to Step B.

### Wave 0 Step B — Linter with auto-fix retry gate

```bash
echo "▶ golangci-lint run ./..."
make lint
```

If `make lint` exits cleanly: print `PASS` and continue to Step C.

If `make lint` exits non-zero:

```bash
echo "  Auto-fixing..."
make lint-fix
make lint
```

Re-run `make lint` after the auto-fix. If lint still exits non-zero: print all remaining issues, then print `ABORT: lint issues remain after auto-fix — resolve manually before releasing` and stop with no file changes.

If the second `make lint` passes: print `PASS (auto-fixed)` and continue to Step C.

### Wave 0 Step C — Integration tests with Docker auto-detect

```bash
echo "▶ test-ci (integration tests)"
```

Check for a Docker socket:

```bash
[ -S /var/run/docker.sock ] || [ -S $HOME/.colima/default/docker.sock ]
```

If no socket is found: print `  WARNING: Docker not detected — skipping integration tests` and continue (non-blocking).

If a socket is found: run `make test-ci`. If `make test-ci` fails: print `ABORT: integration tests failed` and stop with no file changes. If it passes: print `PASS`.

---

After all three sub-steps pass, print `All checks passed — proceeding with release` then continue to Step 1.

## Step 1 — Detect latest tag

```bash
git tag --sort=-v:refname | head -1
```

Store as `$CURRENT_TAG` (e.g. `v0.7.8`). Strip the leading `v` to get `$CURRENT_VERSION` (e.g. `0.7.8`).

If no tags exist, treat current version as `0.0.0`.

## Step 2 — Parse semver

Split `$CURRENT_VERSION` into `MAJOR.MINOR.PATCH`.

## Step 3 — Determine bump type

If `$ARGUMENTS` is one of `major`, `minor`, or `patch`, use it directly.

Otherwise ask the user:

```
Current tag: $CURRENT_TAG
Which part to bump?
  • patch  → vMAJOR.MINOR.(PATCH+1)   [default]
  • minor  → vMAJOR.(MINOR+1).0
  • major  → v(MAJOR+1).0.0
```

Compute `$NEXT_TAG` (e.g. `v0.7.9`).

## Step 4 — Confirm before any changes

Show the plan and ask for confirmation:

```
Ready to release $NEXT_TAG:
  • Update README.md: s/$CURRENT_TAG/$NEXT_TAG/g
  • Update INSTALL.md: s/$CURRENT_TAG/$NEXT_TAG/g
  • git commit -m "chore: bump version to $NEXT_TAG"
  • git tag $NEXT_TAG
  • git push && git push --tags

Proceed? [y/N]
```

If the user says no, abort with no changes made.

## Step 5 — Update README.md and INSTALL.md

Read both files. Replace **all** occurrences of the old version string with `$NEXT_TAG` in each.

The version appears in install URLs like:
```
https://raw.githubusercontent.com/webcane/docker-deploy/v0.8.4/install.sh
```
and inline `INSTALL_VERSION=v0.8.4` env vars.

Use Edit to update each occurrence in both files. Verify no old version string remains after editing.

## Step 6 — Commit

```bash
git add README.md INSTALL.md
git commit -m "chore: bump version to $NEXT_TAG"
```

## Step 7 — Tag

```bash
git tag $NEXT_TAG
```

## Step 8 — Push

```bash
git push && git push --tags
```

If `git push` fails (e.g. no upstream, auth error):
- Leave the local commit and tag in place
- Report the exact error
- Tell the user: `Local commit and tag are ready. Run: git push && git push --tags`

## Step 9 — Report

```
Released $NEXT_TAG
  README.md updated
  INSTALL.md updated
  Commit: {short hash}
  Tag: $NEXT_TAG pushed → GitHub Actions CI/release workflow triggered
```

</process>

<guardrails>
- NEVER push without explicit user confirmation in Step 4
- NEVER create a tag before the README commit succeeds
- If push fails, do NOT delete the local tag — leave it for the user to push manually
- Only README.md and INSTALL.md are modified; no other files change
</guardrails>
