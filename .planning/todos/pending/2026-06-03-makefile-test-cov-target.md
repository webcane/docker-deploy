---
title: Add test-cov Makefile target and wire CI to use it
date: 2026-06-03
priority: low
---

Extract the inline coverage command from `.github/workflows/ci-jobs.yml` into a dedicated Make target.

**Makefile change:**
```makefile
test-cov:
	go test -coverprofile=coverage.out ./...
```

**CI change** (`ci-jobs.yml`, `test` job):
Replace the inline `go test -coverprofile=coverage.out ./...` step with:
```yaml
- name: Run tests
  run: make test-cov
```

The `coverage.out` filename stays as-is — the badge action references it by that name.
