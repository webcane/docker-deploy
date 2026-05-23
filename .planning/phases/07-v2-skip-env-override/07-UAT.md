---
status: complete
phase: 07-v2-skip-env-override
source: [07-01-SUMMARY.md, 07-02-SUMMARY.md]
started: 2026-05-23T00:00:00Z
updated: 2026-05-23T12:00:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: Run `go build ./...` from the repo root — binary compiles without errors. Then run `./docker-deploy --help` (or the built binary) — it prints usage and exits 0 with no panics or stack traces. The `--skip-env` and `--verbose` flags appear in the help output.
result: pass

### 2. Config test suite
expected: Run `go test ./internal/config/... -count=1 -v`. All tests pass including the new TestResolveSkipEnv (5 subtests), TestResolveVerbose (2 subtests), and TestResolveExpandedDefaults. No failures, no panics.
result: pass

### 3. Full regression suite
expected: Run `go test ./... -count=1`. All tests across all packages pass — no regressions in config, filetransfer, compose, or cmd packages from the FlagOpts refactor or verbose param additions.
result: pass

### 4. Expanded default excludes
expected: Build and run a deploy (or dry-run) against a project directory containing `.github/`, `.planning/`, `.vscode/`, `.idea/` subdirectories. These directories are excluded automatically without adding them to deploy.yaml — only project files are transferred.
result: pass

### 5. --verbose flag output
expected: Run deploy with `--verbose`. Stderr shows per-file `  -> <filename>` lines, `[ssh] <cmd>` lines for each remote SSH command, `  → exit 0` (or exit N) after each command, and `  [PASS]` / `  [WARN]` preflight checklist lines. Without `--verbose`, none of these detail lines appear.
result: pass

### 6. --skip-env flag behavior
expected: Run deploy with `--skip-env`. The remote `.env` file is NOT overwritten — its previous contents survive the deploy. Stderr shows `WARNING: .env not uploaded — remote .env left unchanged`. Without `--skip-env`, the .env is uploaded as normal.
result: pass

### 7. Warning rollup (non-verbose mode)
expected: Trigger a non-blocking warning (e.g. a preflight CHECK-03 or CHECK-07 condition). Without `--verbose`, a single rollup line prints: `WARN: there are some warnings during deployment. For more details use --verbose flag`. With `--verbose`, individual warning lines print inline instead of the rollup.
result: pass

## Summary

total: 7
passed: 7
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
