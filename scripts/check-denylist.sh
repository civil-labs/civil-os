#!/bin/sh
# Fails if any tracked file contains a customer- or jurisdiction-specific
# term. The terms come from the CUSTOMER_DENYLIST secret (one per line,
# case-insensitive, fixed strings) so the list itself never enters this
# public repo. Without the secret (forks, local runs) the check is skipped.
#
# Output names files only, never the matching line or term: CI logs on a
# public repo are public.
set -eu

if [ -z "${CUSTOMER_DENYLIST:-}" ]; then
  echo "::warning::CUSTOMER_DENYLIST is not set; skipping the customer-data check"
  exit 0
fi

patterns=$(mktemp)
trap 'rm -f "$patterns"' EXIT
# Drop blank lines and surrounding whitespace.
printf '%s\n' "$CUSTOMER_DENYLIST" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//' | grep -v '^$' > "$patterns" || true

if [ ! -s "$patterns" ]; then
  echo "::warning::CUSTOMER_DENYLIST has no terms; skipping the customer-data check"
  exit 0
fi

if matches=$(git grep -I -l -i -F -f "$patterns" -- . ':!scripts/check-denylist.sh'); then
  echo "::error::Customer-specific terms found in these files (terms not shown):"
  printf '%s\n' "$matches" | sed 's/^/  /'
  exit 1
fi
echo "No customer-specific terms found ($(wc -l < "$patterns") terms checked)."
