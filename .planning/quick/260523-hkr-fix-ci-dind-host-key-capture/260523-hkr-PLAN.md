---
quick_id: 260523-hkr
slug: fix-ci-dind-host-key-capture
description: Fix CI integration test failure — DinD SSH host key capture race condition
date: 2026-05-23
must_haves:
  truths:
    - captureHostKeyFromContainer retries on failure instead of failing immediately
    - retry loop runs for up to 30 seconds with 500ms pauses
    - context cancellation is respected during retries
  artifacts:
    - integration/helpers_test.go (captureHostKeyFromContainer modified)
---

# Plan: Fix CI DinD SSH host key capture race condition

## Root Cause

`wait.ForListeningPort("22/tcp")` fires when the TCP port is open, but sshd may not yet
be ready to complete an SSH handshake (especially on slow/loaded CI). A single dial attempt
means any timing gap → "failed to capture SSH host key from DinD container".

Works locally because local Docker is faster and sshd is ready almost immediately after the
port opens.

## Task 1: Add retry loop to captureHostKeyFromContainer

**File:** `integration/helpers_test.go`
**Action:** Replace the single-attempt logic in `captureHostKeyFromContainer` with a retry
loop that retries every 500ms for up to 30 seconds, respecting context cancellation.
**Verify:** CI passes; error message updated to include timeout duration.
**Done:** `captureHostKeyFromContainer` retries until captured or deadline exceeded.
