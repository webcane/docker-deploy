---
created: 2026-06-03T00:00:00Z
title: Add make test command with pass/fail summary output
area: tooling
files: []
---

## Problem

Running `go test -v ./...` gives verbose output but no summary count at the end. During development it's useful to immediately see how many tests passed/failed without scanning through all the output.

## Solution

Add a `test` target to the Makefile that wraps `go test -v ./...` and appends a summary block with total/passed/failed counts:

```bash
#!/bin/bash
output=$(go test -v ./...)
echo "$output"

passed=$(echo "$output" | grep -c "^--- PASS")
failed=$(echo "$output" | grep -c "^--- FAIL")
total=$((passed + failed))

echo ""
echo "================================================"
echo "Total: $total"
echo "Passed: $passed"
echo "Failed: $failed"
echo "================================================"
```

Implement as a Makefile target (e.g. `make test`) using a shell script snippet or a small wrapper script in `scripts/`.
