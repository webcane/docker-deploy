---
plan: "12-02"
phase: "12"
status: complete
completed: 2026-05-24
---

# Plan 12-02 Summary: Restructure README

## What Was Built

1. Tightened "What is docker-deploy?" value proposition to 60 words (≤100 limit).
2. Restructured Installation section: removed Homebrew, Manual binary, and go install blocks; kept only the install script (two curl commands) and added a one-line link to INSTALL.md.

## Key Files

- `README.md` — value prop paragraph rewritten; Installation section reduced from 4 options to 1 + INSTALL.md link

## Verification

- Word count in value prop: 60 (≤100) ✓
- `grep -c "INSTALL.md" README.md` → 1 ✓
- `grep -c "Option 1|Option 2|Option 3|Option 4" README.md` → 0 ✓
- `grep -c "brew tap|brew install" README.md` → 0 ✓
- `grep -c "curl -fsSL" README.md` → 2 ✓
- `grep -c "### Install script" README.md` → 1 ✓

## Self-Check: PASSED
