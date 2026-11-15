#!/bin/sh
# Reads changed paths, one per line, on stdin. Prints "true" when any path can
# change the public API (api/proto/public/..., api/buf.yaml, api/buf.lock), and
# "false" otherwise. The breaking-change check skips buf breaking only on
# "false"; see docs/docs-only-ci/spec.md.
#
# An empty list prints "true": with nothing to go on, run the check.
set -eu

paths=$(cat)
if [ -z "$paths" ]; then
  echo true
elif printf '%s\n' "$paths" | grep -qE '^api/proto/public/|^api/buf\.(yaml|lock)$'; then
  echo true
else
  echo false
fi
