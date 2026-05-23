---
quick_id: 260523-cos
slug: fix-goreleaser-cosign-ci
description: Fix goreleaser cosign not found in CI release pipeline
date: 2026-05-23
status: planned
must_haves:
  truths:
    - sigstore/cosign-installer step added to release.yml before goreleaser step
    - release workflow retains id-token write permission (already present)
  artifacts:
    - .github/workflows/release.yml
---

# Quick Task 260523-cos: Fix goreleaser cosign not found in CI

## Goal

GoReleaser release pipeline fails with `exec: "cosign": executable file not found in $PATH`.
The `.goreleaser.yaml` signs the checksums file using cosign keyless signing, but the
`release.yml` workflow never installs cosign. The `id-token: write` permission is already
in place for keyless signing — just the installer step is missing.

## Tasks

### Task 1: Add cosign-installer to release workflow

**File:** `.github/workflows/release.yml`

**Action:** Insert `sigstore/cosign-installer@v3` step before the `goreleaser/goreleaser-action@v6` step.

```yaml
      - uses: sigstore/cosign-installer@v3
```

No additional config needed — cosign-installer defaults to the latest stable cosign version,
and keyless signing is driven by the `id-token: write` permission already in the job.

**Verify:** Step appears before goreleaser step in release.yml.

**Done:** `.github/workflows/release.yml` contains `sigstore/cosign-installer@v3`.
