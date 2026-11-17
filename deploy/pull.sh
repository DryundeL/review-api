#!/bin/sh
# Usage: pull.sh [commit-sha]  — activate a release (default: current).
# Only the current and previous releases and their images are kept.
set -eu
[ "$(id -un)" = wapp ] || { echo 'Run as wapp only (without sudo).' >&2; exit 1; }
[ "$#" -le 1 ] || { echo 'Usage: pull.sh [commit-sha]' >&2; exit 1; }

project=review-api
image=ghcr.io/dryundel/review-api
root="/home/wapp/apps/$project"
share=/home/wapp/.local/share
state="$share/$project"
unset DOCKER_CONTEXT
export DOCKER_HOST=unix:///run/user/1000/docker.sock

die() { echo "$*" >&2; exit 1; }
valid_sha() {
  case "$1" in ''|*[!0-9a-f]*) return 1 ;; esac
  [ "${#1}" -eq 40 ]
}
release_of() {
  [ -L "$state/$1" ] || return 0
  sha=$(basename "$(readlink "$state/$1")")
  valid_sha "$sha" || die "Invalid $1 release reference."
  echo "$sha"
}

# Shared with review-app. activate.sh already holds it during CI deploys.
if [ "${REVIEW_DEPLOY_LOCKED:-}" != 1 ]; then
  exec 9>>"$share/review-deploy.lock"
  flock -w 900 9 || die 'Timed out waiting for the Review deployment lock.'
fi

revision=${1:-$(release_of current)}
valid_sha "$revision" || die 'Invalid commit SHA.'
release="$state/releases/$revision"
[ "$(cat "$release/.release-version")" = "$revision" ] || die "Release $revision is not installed."
if ! docker image inspect "$image:$revision" >/dev/null 2>&1; then
  # Releases deployed before GHCR were loaded as review-api:<sha>.
  docker image inspect "$project:$revision" >/dev/null 2>&1 || die "Image $image:$revision is not available."
  docker tag "$project:$revision" "$image:$revision"
fi
test -f "$root/.env" || die "Missing $root/.env."

export APP_VERSION="$revision" APP_ENV_FILE="$root/.env"
cd "$release"
compose() { docker compose --env-file "$root/.env" -f "$release/compose.yaml" "$@"; }
compose config --quiet
compose up -d --no-build --no-recreate --wait --wait-timeout 120 postgres redis
compose stop worker bot
# API applies migrations before becoming ready. Do not restart the databases.
compose up -d --no-build --pull never --no-deps --force-recreate --wait --wait-timeout 180 app
compose up -d --no-build --pull never --no-deps --force-recreate --wait --wait-timeout 90 worker bot

set_ref() {
  ln -sfn "$state/releases/$2" "$state/$1.next"
  mv -Tf "$state/$1.next" "$state/$1"
}
current=$(release_of current)
previous=$(release_of previous)
if [ -n "$current" ] && [ "$current" != "$revision" ]; then
  previous=$current
  set_ref previous "$previous"
fi
set_ref current "$revision"

# Keep the current and previous releases, and any release a container still
# bind-mounts (Redis keeps the entrypoint of the release it was created from).
if ids=$(docker container ls -aq) &&
  mounts=$(if [ -n "$ids" ]; then docker container inspect --format '{{range .Mounts}}{{println .Source}}{{end}}' $ids; fi); then
  for dir in "$state"/releases/*; do
    name=${dir##*/}
    valid_sha "$name" || continue
    case "$name" in "$revision"|"$previous") continue ;; esac
    case "
$mounts" in *"
$dir/"*) continue ;; esac
    rm -rf -- "$dir"
  done
else
  echo 'Warning: release cleanup skipped.' >&2
fi

# Docker refuses to remove an image used by a container, so a failed removal
# is only reported.
for repo in "$image" "$project"; do
  docker image ls --format '{{.Tag}}' "$repo" | while read -r tag; do
    case "$tag" in "$revision"|"$previous"|'<none>') continue ;; esac
    docker image rm "$repo:$tag" >/dev/null || echo "Warning: kept $repo:$tag." >&2
  done
done
docker image prune -f >/dev/null || echo 'Warning: dangling image cleanup failed.' >&2

# Bundles left by interrupted deployments.
[ ! -d "$state/incoming" ] ||
  find "$state/incoming" -mindepth 1 -maxdepth 1 -mmin +1440 -exec rm -rf -- {} + ||
  echo 'Warning: incoming cleanup failed.' >&2

compose ps
