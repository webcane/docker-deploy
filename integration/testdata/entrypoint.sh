#!/bin/sh
set -e

# Start Docker daemon in background
dockerd --host=unix:///var/run/docker.sock &

# Wait for Docker socket to be ready
for i in $(seq 1 30); do
    if docker info >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

# Fail fast if Docker daemon did not start within the 30-second window.
# Without this guard, sshd starts anyway and all Docker-dependent tests fail
# with opaque SSH errors rather than a clear startup failure message.
if ! docker info >/dev/null 2>&1; then
    echo "ERROR: Docker daemon failed to start within 30 seconds" >&2
    exit 1
fi

# Start sshd in foreground
exec /usr/sbin/sshd -D -e
