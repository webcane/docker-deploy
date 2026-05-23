---
created: 2026-05-23T06:54:13Z
title: Add `docker deploy --version` support
area: tooling
files: []
---

## Problem

The `docker deploy --version` flag (or subcommand) is not yet implemented. Users expect CLI tools to support `--version` to confirm which release is installed. Without it, users have no quick way to verify the installed binary version.

## Solution

Wire up a `--version` flag (or `version` subcommand) to the Cobra CLI that prints the build-stamped version string. Version should be injected at build time via `-ldflags "-X main.version=..."` so release binaries carry the correct tag.
