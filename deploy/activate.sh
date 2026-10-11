#!/bin/sh
# Executed over SSH by GitHub Actions; receives a tested image and configuration.
set -eu
revision=${1:?Commit SHA required}
case "$revision" in
  *[!0-9a-f]*) echo 'Invalid commit SHA.' >&2; exit 1 ;;
esac
[ "${#revision}" -eq 40 ] || exit 1

root=/home/wapp/apps/review-api
release="$root/releases/$revision"
export DOCKER_CONTEXT=rootless
export DEPLOY_LOCK_FILE="$root/.deploy.lock"
test -f "$root/.env"
mkdir -p "$release"
tar -xzf "$root/incoming/$revision/release.tar.gz" -C "$release"
test "$(cat "$release/.release-version")" = "$revision"
ln -sfn "$root/.env" "$release/.env"
docker load -i "$release/image.tar.gz"
sh "$release/deploy/deploy.sh" --no-build
# Promote only after readiness succeeds. Previous releases remain available.
ln -sfn "$release" "$root/current"
rm -f "$release/image.tar.gz" "$root/incoming/$revision/release.tar.gz"
