#!/bin/sh
# Usage: is-breaking-bump.sh OLD NEW
# Prints "true" when NEW is a breaking version bump over OLD, "false" otherwise.
# Before 1.0 a breaking change bumps the minor version; from 1.0, the major
# (docs/breaking-check-baseline/spec.md, release-please's bump-minor-pre-major).
#
# Both versions must be plain X.Y.Z. Anything else exits 2: the release-time
# check must fail, not guess.
set -eu

if [ "$#" -ne 2 ]; then
  echo "usage: is-breaking-bump.sh OLD NEW" >&2
  exit 2
fi

for v in "$1" "$2"; do
  if ! printf '%s\n' "$v" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
    echo "is-breaking-bump.sh: not an X.Y.Z version: $v" >&2
    exit 2
  fi
done

old_major=${1%%.*}
old_rest=${1#*.}
old_minor=${old_rest%%.*}
new_major=${2%%.*}
new_rest=${2#*.}
new_minor=${new_rest%%.*}

if [ "$new_major" -gt "$old_major" ]; then
  echo true
elif [ "$old_major" -eq 0 ] && [ "$new_major" -eq 0 ] && [ "$new_minor" -gt "$old_minor" ]; then
  echo true
else
  echo false
fi
