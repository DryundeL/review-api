#!/bin/sh
# Usage:
#   pull.sh                              restart the current release
#   pull.sh <SHA>                        switch to an already loaded release (rollback)
#   pull.sh --activate <SHA> <UPLOAD_ID> install an uploaded release (CI)
set -eu
umask 077
[ "$(id -un)" = wapp ] || { echo 'Run as wapp only.' >&2; exit 1; }

project=review-api
root="/home/wapp/apps/$project"
share=/home/wapp/.local/share
state="$share/$project"
unset DOCKER_CONTEXT
export DOCKER_HOST=unix:///run/user/1000/docker.sock

# CI streams this script through stdin, so no child may read it.
docker() { command docker "$@" </dev/null; }
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

upload=
case "${1:-}" in
  --activate)
    [ "$#" -eq 3 ] || die 'Usage: pull.sh --activate <SHA> <UPLOAD_ID>'
    case "$3" in *[!0-9.]*|.*|*.|*..*|'') die 'Invalid upload id.' ;; esac
    revision=$2
    upload="$state/incoming/$revision.$3/release.tar.gz"
    ;;
  *)
    [ "$#" -le 1 ] || die 'Usage: pull.sh [<SHA>]'
    revision=${1:-$(release_of current)}
    ;;
esac
valid_sha "$revision" || die 'Invalid commit SHA.'

mkdir -p "$root" "$state/releases"
# Shared with the other Review project. Never remove this lock file.
exec 9>>"$share/review-deploy.lock"
flock -w 900 9 || die 'Timed out waiting for the Review deployment lock.'

docker info --format '{{range .SecurityOptions}}{{println .}}{{end}}' | grep -qx name=rootless ||
  die 'Rootless Docker is unavailable.'
test -f "$root/.env" || die "Missing $root/.env."

current=$(release_of current)
previous=$(release_of previous)
release="$state/releases/$revision"

stage=
trap '[ -z "$stage" ] || rm -rf -- "$stage"' EXIT
if [ -n "$upload" ]; then
  stage=$(mktemp -d "$state/releases/.stage.XXXXXXXX")
  tar --no-same-owner --no-same-permissions -xzf "$upload" -C "$stage"
  [ "$(cat "$stage/.release-version")" = "$revision" ] || die 'Release version mismatch.'
  if [ ! -d "$release" ]; then
    mkdir "$stage/config"
    mv -- "$stage/compose.yaml" "$stage/.release-version" "$stage/deploy" "$stage/config/"
    mv -T -- "$stage/config" "$release"
  fi
  # A rerun of the same commit must not replace an already deployed tag.
  docker image inspect "$project:$revision" >/dev/null 2>&1 ||
    docker load -q -i "$stage/image.tar.gz"
fi
[ "$(cat "$release/.release-version")" = "$revision" ] || die "Release $revision is not installed."
docker image inspect "$project:$revision" >/dev/null 2>&1 || die "Image $project:$revision is not loaded."

export APP_VERSION="$revision" APP_ENV_FILE="$root/.env"
cd "$release"
compose() { docker compose --env-file "$root/.env" -f "$release/compose.yaml" "$@"; }
compose config --quiet
compose up -d --no-build --no-recreate --wait --wait-timeout 120 postgres redis
compose stop worker bot
# API applies migrations before becoming ready. Do not restart the databases.
compose up -d --no-build --pull never --no-deps --force-recreate --wait --wait-timeout 120 app
compose up -d --no-build --pull never --no-deps --force-recreate --wait --wait-timeout 90 worker bot

set_ref() {
  ln -sfn "$state/releases/$2" "$state/$1.next"
  mv -Tf "$state/$1.next" "$state/$1"
}
if [ -n "$current" ] && [ "$current" != "$revision" ]; then
  previous=$current
  set_ref previous "$previous"
fi
set_ref current "$revision"

if [ -n "$upload" ]; then
  install -m 700 "$stage/pull.sh" "$root/pull.sh.next"
  mv -Tf "$root/pull.sh.next" "$root/pull.sh"
  rm -rf -- "${upload%/*}"
fi

# Keep only the current and previous images. Docker refuses to remove an image
# used by any container, so a failed removal is reported and skipped.
docker image ls --format '{{.Tag}}' "$project" | while read -r tag; do
  case "$tag" in "$revision"|"$previous"|'<none>') continue ;; esac
  docker image rm "$project:$tag" >/dev/null || echo "Warning: kept $project:$tag." >&2
done
docker image prune -f >/dev/null || echo 'Warning: dangling image cleanup failed.' >&2

# Uploads of failed deployments.
[ ! -d "$state/incoming" ] ||
  find "$state/incoming" -mindepth 1 -maxdepth 1 -type d -mmin +1440 -exec rm -rf -- {} + ||
  echo 'Warning: incoming cleanup failed.' >&2

compose ps
