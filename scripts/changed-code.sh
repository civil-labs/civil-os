#!/bin/sh
# Reads changed paths, one per line, on stdin. Prints "false" when every path
# is documentation (under docs/ or ending in .md, case-sensitive) and "true"
# otherwise. CI skips the code jobs only on "false"; see
# docs/docs-only-ci/spec.md.
#
# Anything uncertain counts as code, because running the tests is the safe
# side: an empty list, README.MD, notes.markdown.
set -eu

paths=$(cat)
if [ -z "$paths" ]; then
  echo true
elif printf '%s\n' "$paths" | grep -qvE '^docs/|\.md$'; then
  echo true
else
  echo false
fi
