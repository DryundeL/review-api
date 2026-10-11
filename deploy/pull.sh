#!/bin/sh
set -eu
if [ "$(id -un)" != wapp ]; then
  echo 'Run this script as wapp only (without sudo).' >&2
  exit 1
fi
[ "$#" -le 1 ] || { echo 'Usage: pull.sh [commit-sha]' >&2; exit 1; }
root=/home/wapp/apps/review-api
state=/home/wapp/.local/share/review-api
export DOCKER_CONTEXT=rootless
# One lock for both manual restarts and GitHub Actions deployments.
exec 9>"$state/deploy.lock"
flock -n 9 || { echo 'Another deployment is running.' >&2; exit 1; }
revision=${1:-$(cat "$state/current/.release-version")}
case "$revision" in
  ''|*[!0-9a-f]*) echo 'Invalid commit SHA.' >&2; exit 1 ;;
esac
[ "${#revision}" -eq 40 ] || { echo 'Invalid commit SHA length.' >&2; exit 1; }
release="$state/releases/$revision"
test -f "$root/.env"
test "$(cat "$release/.release-version")" = "$revision"
export APP_VERSION="$revision"
export APP_ENV_FILE="$root/.env"
cd "$release"
compose() {
  docker compose --env-file "$root/.env" -f "$release/compose.yaml" "$@"
}
compose config --quiet
docker image inspect "review-api:$revision" >/dev/null
compose up -d --no-build --wait --wait-timeout 120 postgres redis
compose stop worker
# API applies migrations before becoming ready. Do not restart the databases.
compose up -d --no-build --no-deps --force-recreate --wait --wait-timeout 120 app
compose up -d --no-build --no-deps --force-recreate --wait --wait-timeout 60 worker
ln -sfn "$release" "$state/current.next"
mv -Tf "$state/current.next" "$state/current"
compose ps
