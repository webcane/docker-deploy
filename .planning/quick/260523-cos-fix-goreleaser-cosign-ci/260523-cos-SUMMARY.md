---
quick_id: 260523-cos
slug: fix-goreleaser-cosign-ci
status: complete
---

# Quick Task 260523-cos: Fix goreleaser cosign not found in CI

## What was done

Added `sigstore/cosign-installer@v3` step to `.github/workflows/release.yml` before the
goreleaser step. The `.goreleaser.yaml` already configured keyless cosign signing of the
checksums file, and the job already had `id-token: write` permission — the only missing
piece was the installer step that puts `cosign` on the runner's PATH.

## Files changed

- `.github/workflows/release.yml` — added `sigstore/cosign-installer@v3` step
