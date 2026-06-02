---
title: Pin integration test image tags
date: 2026-05-26
priority: medium
---

## Problem

Integration tests always pull images on every local run because testcontainers-go re-checks
mutable tags (`:latest`, `:alpine`) against the registry by default. This adds unnecessary
latency for local development with Colima.

## What to change

**`integration/helpers_test.go:65`**
```
lscr.io/linuxserver/openssh-server:latest
→ lscr.io/linuxserver/openssh-server:<specific-release-tag>
```
Find the current release at https://github.com/linuxserver/docker-openssh-server/releases

**`integration/compose_test.go:29`**
```
image: nginx:alpine
→ image: nginx:1.27-alpine   (or latest stable at time of pinning)
```
The TODO comment on line 19 already calls this out.

## Notes

- After pinning, consider adding `ImagePullPolicy: testcontainers.NeverPullImage` to
  `ContainerRequest` structs as a belt-and-suspenders measure for local dev speed.
- CI doesn't need special treatment — pinned tags will be pulled once and cached by the
  runner's Docker layer cache if configured.
- Periodically bump these tags when upstream security patches land.
