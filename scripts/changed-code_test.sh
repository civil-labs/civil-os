#!/bin/sh
# Table tests for changed-code.sh and touches-public-api.sh. Run from anywhere:
#   sh scripts/changed-code_test.sh
# CI runs it before trusting either script.
set -eu

dir=$(dirname "$0")
failed=0

# check SCRIPT EXPECTED PATHS... (each argument is one line of input)
check() {
  script=$1 want=$2
  shift 2
  if [ "$#" -eq 0 ]; then
    got=$(printf '' | sh "$dir/$script")
  else
    got=$(printf '%s\n' "$@" | sh "$dir/$script")
  fi
  if [ "$got" != "$want" ]; then
    echo "FAIL $script [$*]: got $got, want $want"
    failed=1
  fi
}

# Docs-only rule (docs/docs-only-ci/spec.md, "Classification cases").
check changed-code.sh false docs/x/intent.md
check changed-code.sh false docs/diagram.svg
check changed-code.sh false README.md api/proto/public/README.md CHANGELOG.md
check changed-code.sh true docs/x/spec.md services/gateway/main.go
check changed-code.sh true .github/workflows/ci.yml
check changed-code.sh true scripts/changed-code.sh
check changed-code.sh true mise.toml
check changed-code.sh true notes.markdown
check changed-code.sh true README.MD
check changed-code.sh true docsfoo/x.txt
check changed-code.sh true
# A rename lists both names: code moved into docs/ is still code.
check changed-code.sh true docs/x.go services/x.go

# Public API rule (docs/docs-only-ci/plan.md, step 1).
check touches-public-api.sh true api/proto/public/civil/public/parcels/v1/parcels.proto
check touches-public-api.sh true api/buf.yaml
check touches-public-api.sh true api/buf.lock
check touches-public-api.sh false api/proto/mesh/civil/mesh/parcels/v1/parcels.proto
check touches-public-api.sh false api/buf.gen.public.yaml
check touches-public-api.sh false services/gateway/main.go docs/x.md
check touches-public-api.sh true api/proto/public/README.md
check touches-public-api.sh true

if [ "$failed" -ne 0 ]; then
  exit 1
fi
echo "changed-code_test: all cases pass"
