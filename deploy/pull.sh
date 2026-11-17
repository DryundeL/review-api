#!/bin/sh
set -eu
umask 077
[ "$(id -un)" = wapp ] || { echo 'Run as wapp only.' >&2; exit 1; }
project=review-api
root="/home/wapp/apps/$project"
share=/home/wapp/.local/share
state="$share/$project"
socket=/run/user/1000/docker.sock
unset DOCKER_CONTEXT
export DOCKER_HOST="unix://$socket"

valid_sha() {
  case "$1" in ''|*[!0-9a-f]*) return 1 ;; esac
  [ "${#1}" -eq 40 ]
}

upload=
revision=
case "${1:-}" in
  --activate)
    [ "$#" -eq 3 ] || exit 1
    revision=$2
    valid_sha "$revision" || exit 1
    case "$3" in *.*.*|.*|*.|*[!0-9.]*) exit 1 ;; *.*) ;; *) exit 1 ;; esac
    upload="$state/incoming/$revision.$3/release.tar.gz"
    ;;
  *) [ "$#" -le 1 ] || exit 1 ;;
esac

mkdir -p "$root" "$state/releases"
# Both projects use the same inode. Never remove this lock file.
exec 9>>"$share/review-deploy.lock"
flock -w 900 9 || { echo 'Timed out waiting for the Review deployment lock.' >&2; exit 1; }
# Coordinate with installed pull.sh versions during rollout.
exec 8>>"$state/deploy.lock"
flock -w 60 8 || { echo 'An older pull.sh is still running.' >&2; exit 1; }

pending="$share/review-docker.pending"
if [ -e "$pending" ]; then
  echo "Unconfirmed Docker operation. Check the daemon and $pending before retrying." >&2
  exit 1
fi

docker_read() {
  timeout -k 5s 15s docker --host "$DOCKER_HOST" "$@"
}

docker_change() {
  duration=$1
  shift
  # Killing the CLI may leave a request running inside dockerd.
  printf '%s %s %s\n' "$project" "$revision" "$1" > "$pending" || return 1
  status=0
  timeout -k 5s "$duration" docker --host "$DOCKER_HOST" "$@" || status=$?
  case "$status" in
    124|129|130|137|143) echo "Docker operation timed out or was interrupted. Review deployments are blocked by $pending." >&2 ;;
    *) rm -f -- "$pending" || return 1 ;;
  esac
  return "$status"
}

revision=${revision:-${1:-$(cat "$state/current/.release-version")}}
valid_sha "$revision" || { echo 'Invalid commit SHA.' >&2; exit 1; }
release="$state/releases/$revision"
previous=
current=
for ref in current previous; do
  if [ -L "$state/$ref" ]; then
    target=$(readlink "$state/$ref")
    sha=${target##*/}
    valid_sha "$sha" && [ "$target" = "$state/releases/$sha" ] &&
      [ "$(cat "$target/.release-version")" = "$sha" ] ||
      { echo "Invalid $ref release reference." >&2; exit 1; }
    case "$ref" in current) current=$sha ;; previous) previous=$sha ;; esac
  fi
done

test -S "$socket"
docker_read info --format '{{range .SecurityOptions}}{{println .}}{{end}}' | grep -qx 'name=rootless' ||
  { echo 'Rootless Docker is unavailable or its API timed out.' >&2; exit 1; }
test -f "$root/.env"
stage=
trap 'if [ -n "$stage" ]; then rm -rf -- "$stage"; fi' EXIT
if [ -n "$upload" ]; then
  stage=$(mktemp -d "$state/releases/.stage.XXXXXXXX")
  tar --no-same-owner --no-same-permissions -xzf "$upload" -C "$stage"
  test "$(cat "$stage/.release-version")" = "$revision"
  if [ ! -d "$release" ]; then
    mkdir "$stage/config"
    mv -- "$stage/compose.yaml" "$stage/.release-version" "$stage/deploy" "$stage/config/"
    mv -T -- "$stage/config" "$release"
  fi
  # A rerun of the same commit must not replace its already deployed tag.
  loaded=$(docker_read image ls --quiet "$project:$revision")
  if [ -z "$loaded" ]; then
    docker_change 600s load -i "$stage/image.tar.gz"
  fi
fi
test "$(cat "$release/.release-version")" = "$revision"
export APP_VERSION="$revision"
export APP_ENV_FILE="$root/.env"
cd "$release"
compose() {
  limit=$1
  shift
  docker_change "$limit" compose --env-file "$root/.env" -f "$release/compose.yaml" "$@"
}
docker_read compose --env-file "$root/.env" -f "$release/compose.yaml" config --quiet
docker_read image inspect "$project:$revision" >/dev/null
compose 240s up -d --no-build --no-recreate --wait --wait-timeout 120 postgres redis
compose 60s stop worker bot
# API applies migrations before becoming ready. Do not restart the databases.
compose 240s up -d --no-build --pull never --no-deps --force-recreate --wait --wait-timeout 120 app
compose 180s up -d --no-build --pull never --no-deps --force-recreate --wait --wait-timeout 90 worker
compose 180s up -d --no-build --pull never --no-deps --force-recreate --wait --wait-timeout 90 bot

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
  rm -f -- "$upload"
  rmdir -- "${upload%/*}"
fi

cleanup_images() {
  # Require a usable rollback image before deleting any tags.
  [ -n "$previous" ] && [ "$previous" != "$revision" ] || return 0
  docker_read image inspect "$project:$previous" >/dev/null || return 1
  images=$(docker_read image ls --no-trunc --format '{{.Repository}} {{.Tag}} {{.ID}}' "$project") || return 1
  containers=$(docker_read container ls --all --quiet) || return 1
  used=
  if [ -n "$containers" ]; then
    # Docker emits one validated container ID per line.
    used=$(docker_read container inspect --format '{{.Image}}' $containers) || return 1
  fi

  removed=0
  while read -r repository tag image_id; do
    [ "$repository" = "$project" ] && valid_sha "$tag" || continue
    [ "$tag" != "$revision" ] && [ "$tag" != "$previous" ] || continue
    if printf '%s\n' "$used" | grep -Fxq -- "$image_id"; then
      continue
    fi
    docker_change 10s image rm --no-prune "$project:$tag" || return 1
    removed=$((removed + 1))
    [ "$removed" -lt 10 ] || break
  done <<EOF
$images
EOF
}

# Only regular archive files at known release paths are eligible. Keep Compose
# files and scripts, including the API's Redis entrypoint bind mount.
cleanup_archives() {
  for dir in "$state/releases/"*; do
    [ -d "$dir" ] && [ ! -L "$dir" ] || continue
    sha=${dir##*/}
    valid_sha "$sha" || continue
    [ "$sha" != "$revision" ] && [ "$sha" != "$previous" ] || continue
    find "$dir" -maxdepth 1 -type f \( -name image.tar -o -name image.tar.gz \) -mtime +1 -delete || return 1
  done
  if [ -d "$state/incoming" ]; then
    find "$state/incoming" -regextype posix-extended -mindepth 2 -maxdepth 2 \
      -regex '.*/[0-9a-f]{40}(\.[0-9]+\.[0-9]+)?/release\.tar\.gz' \
      -type f -mtime +1 -delete || return 1
  fi
}

cleanup_archives || echo 'Warning: old release archive cleanup failed.' >&2
cleanup_images || echo 'Warning: old Docker image cleanup stopped.' >&2
