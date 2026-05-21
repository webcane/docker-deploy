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

# Start sshd in foreground
exec /usr/sbin/sshd -D -e
