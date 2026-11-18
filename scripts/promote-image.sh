#!/bin/sh
# Promote one service's main image to a release tag, without rebuilding it
# (docs/image-promotion/spec.md):
#
#   sh scripts/promote-image.sh IMAGE COMMIT_SHA RELEASE_TAG
#
# e.g. ghcr.io/civil-labs/civil-os/gateway <40-hex sha> v1.2.3. It waits for
# IMAGE:sha-<first 7 of COMMIT_SHA>, which the main build pushes, then tags
# that image index RELEASE_TAG and checks both tags resolve to the same digest.
# An existing RELEASE_TAG is left alone: fine if it already has that digest,
# an error if it doesn't.
#
# Prints one Markdown table row (image | tag | digest) for the job summary.
# Exits 0 on success, 1 on a failure (timeout, digest mismatch, registry
# error) and 2 on a usage error.
#
# PROMOTE_INTERVAL (s, default 15) and PROMOTE_TIMEOUT (s, default 1200) bound
# the wait. IMAGETOOLS (default "docker buildx imagetools") is the command used
# for every registry call; the test points it at a fake.
set -eu

usage() {
  echo "usage: promote-image.sh IMAGE COMMIT_SHA RELEASE_TAG (tag vX.Y.Z, 40-hex SHA)" >&2
  exit 2
}

[ "$#" -eq 3 ] || usage
image=$1
sha=$2
tag=$3
printf '%s\n' "$sha" | grep -qE '^[0-9a-f]{40}$' || usage
printf '%s\n' "$tag" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$' || usage
[ -n "$image" ] || usage

interval=${PROMOTE_INTERVAL:-15}
timeout=${PROMOTE_TIMEOUT:-1200}
imagetools=${IMAGETOOLS:-docker buildx imagetools}

short=$(printf '%.7s' "$sha")
source_ref="$image:sha-$short"
release_ref="$image:$tag"

# digest REF: print REF's digest. Exit 0 if found, 3 if the registry says it
# doesn't exist, 1 on any other error (with the error on stderr).
digest() {
  if out=$($imagetools inspect "$1" --format '{{.Manifest.Digest}}' 2>&1); then
    printf '%s\n' "$out"
    return 0
  fi
  case $out in
    *"not found"*) return 3 ;;
  esac
  printf '%s\n' "$out" >&2
  return 1
}

# 1. Wait for the main image.
waited=0
while :; do
  rc=0
  source_digest=$(digest "$source_ref") || rc=$?
  [ "$rc" -eq 0 ] && break
  if [ "$waited" -ge "$timeout" ]; then
    if [ $((timeout % 60)) -eq 0 ]; then after="$((timeout / 60)) min"; else after="${timeout}s"; fi
    echo "$source_ref not found after $after. Did the main Images run for $sha fail? Re-run it, then re-run this job." >&2
    exit 1
  fi
  sleep "$interval"
  # With PROMOTE_INTERVAL=0 (tests), each poll counts as one second.
  if [ "$interval" -gt 0 ]; then waited=$((waited + interval)); else waited=$((waited + 1)); fi
done

# 2. Never move an existing release tag.
rc=0
release_digest=$(digest "$release_ref") || rc=$?
case $rc in
  0)
    if [ "$release_digest" = "$source_digest" ]; then
      printf '| %s | %s | %s |\n' "$image" "$tag" "$source_digest"
      exit 0
    fi
    echo "$release_ref already exists with $release_digest, not $source_digest from $source_ref. Not moving a published tag." >&2
    exit 1
    ;;
  3) ;;
  *)
    echo "could not check whether $release_ref exists" >&2
    exit 1
    ;;
esac

# 3. Promote, then check the digests match.
$imagetools create --tag "$release_ref" "$source_ref" >&2
release_digest=$(digest "$release_ref") || {
  echo "$release_ref not readable after promotion" >&2
  exit 1
}
if [ "$release_digest" != "$source_digest" ]; then
  echo "$release_ref has $release_digest after promotion, but $source_ref has $source_digest" >&2
  exit 1
fi
printf '| %s | %s | %s |\n' "$image" "$tag" "$source_digest"
