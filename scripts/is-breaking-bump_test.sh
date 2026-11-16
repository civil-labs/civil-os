#!/bin/sh
# Table tests for is-breaking-bump.sh. Run from anywhere:
#   sh scripts/is-breaking-bump_test.sh
# The release-time breaking check runs it before trusting the script.
set -eu

dir=$(dirname "$0")
failed=0

# check EXPECTED ARGS... ; EXPECTED is true, false or exit=N.
check() {
  want=$1
  shift
  if got=$(sh "$dir/is-breaking-bump.sh" "$@" 2>/dev/null); then
    :
  else
    got="exit=$?"
  fi
  if [ "$got" != "$want" ]; then
    echo "FAIL is-breaking-bump.sh [$*]: got $got, want $want"
    failed=1
  fi
}

# docs/breaking-check-baseline/spec.md, "Version bump cases".
check true 0.2.0 0.3.0
check false 0.2.0 0.2.1
check true 0.2.0 1.0.0
check true 1.4.2 2.0.0
check false 1.4.2 1.5.0
check true 0.9.0 0.10.0
check false 0.2.0 0.2.0
check exit=2 0.2.0 garbage
# docs/breaking-check-baseline/plan.md, step 1.
check false 1.0.0 1.0.1
check exit=2 0.2.0
check exit=2 0.2.0 0.3.0-rc.1

if [ "$failed" -ne 0 ]; then
  exit 1
fi
echo "is-breaking-bump_test: all cases pass"
