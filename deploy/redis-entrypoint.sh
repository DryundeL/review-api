#!/bin/sh
set -eu
umask 077
username=${REDIS_USER:-default}
case "$username" in
  *[!a-zA-Z0-9_-]*) echo 'REDIS_USER contains invalid characters.' >&2; exit 1 ;;
esac
case "${REDIS_PASSWORD:-}" in
  '')
    [ "$username" = default ] || { echo 'Custom REDIS_USER requires REDIS_PASSWORD.' >&2; exit 1; }
    printf 'user default on nopass ~* &* +@all\n' > /tmp/review-users.acl
    ;;
  *)
    password_hash=$(printf '%s' "$REDIS_PASSWORD" | sha256sum)
    password_hash=${password_hash%% *}
    {
      [ "$username" = default ] || printf 'user default off\n'
      printf 'user %s on #%s ~* &* +@all\n' "$username" "$password_hash"
    } > /tmp/review-users.acl
    ;;
esac
exec redis-server --port "${REDIS_PORT:-6379}" --aclfile /tmp/review-users.acl \
  --appendonly yes --maxmemory 128mb --maxmemory-policy noeviction
