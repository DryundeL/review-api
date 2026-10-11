#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
case "${1:-}" in
  '') build=true ;;
  --no-build) build=false ;;
  *) echo 'Usage: deploy.sh [--no-build]' >&2; exit 1 ;;
esac
if [ -f .release-version ]; then
  APP_VERSION=$(cat .release-version)
  case "$APP_VERSION" in
    ''|*[!0-9a-f]*) echo 'Invalid release version.' >&2; exit 1 ;;
  esac
  [ "${#APP_VERSION}" -eq 40 ] || exit 1
  export APP_VERSION
fi
if [ ! -f .env ]; then
  echo 'Create .env from deploy/env.example and set the required values.' >&2
  exit 1
fi
chmod 600 .env
# Serialise deployments on this host; the lock is released even on failure.
exec 9>"${DEPLOY_LOCK_FILE:-.deploy.lock}"
flock -n 9 || { echo 'Another deployment is running.' >&2; exit 1; }
docker compose config --quiet
if "$build"; then
  docker compose build app
else
  docker image inspect "review-api:${APP_VERSION:?Release version required}" >/dev/null
fi
# API applies embedded migrations before serving requests. Stop the old worker
# and bot first so that they cannot process records against a changing schema.
docker compose stop worker bot
docker compose up -d --no-build --wait --wait-timeout 120 app
# Recreate worker and bot even when reusing the local image tag.
docker compose up -d --no-build --force-recreate --wait --wait-timeout 60 worker
docker compose up -d --no-build --no-deps --force-recreate --wait --wait-timeout 60 bot
docker compose ps
