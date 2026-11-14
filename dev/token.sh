#!/bin/sh
# Prints an ID token for the local test user (dev/dex.yaml) on stdout, and
# nothing else, so `TOKEN=$(mise run token)` works. Local stack only.
set -eu
issuer="${CIVIL_OS_E2E_ISSUER_URL:-http://localhost:5556/dex}"

body=$(curl -sS --connect-timeout 5 --max-time 15 "$issuer/token" \
  -d grant_type=password -d scope='openid email profile' \
  -d username=dev@example.com -d password=civil-os-dev-password \
  -d client_id=civil-os-dev -d client_secret=civil-os-dev-secret) || {
  echo "token: could not reach Dex at $issuer; is the stack up? (mise run up)" >&2
  exit 1
}
token=$(printf '%s' "$body" | sed -n -E 's/.*"id_token":"([^"]+)".*/\1/p')
if [ -z "$token" ]; then
  echo "token: no id_token in Dex response: $body" >&2
  exit 1
fi
printf '%s\n' "$token"
