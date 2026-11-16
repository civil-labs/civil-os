#!/bin/sh
# Usage: check-migrations-size.sh DIR
#
# Checks that the files in DIR fit in one ConfigMap. Each file becomes one
# `data` value, so the ConfigMap's data size is the sum of the file sizes.
# Kubernetes caps ConfigMap data at 1 MiB.
#
#   up to 768 KiB    prints the size and exits 0
#   over 768 KiB     also prints a GitHub Actions warning and exits 0, which
#                    leaves time to squash the history into a new baseline
#   over 1 MiB       prints a GitHub Actions error and exits 1
set -eu

dir=${1:?usage: check-migrations-size.sh DIR}
warn=786432   # 768 KiB
limit=1048576 # 1 MiB

if [ ! -d "$dir" ]; then
  echo "::error::$dir is not a directory"
  exit 1
fi

# wc -c reads byte counts portably; awk sums them (0 for an empty directory).
size=$(find "$dir" -maxdepth 1 -type f -exec wc -c {} + |
  awk '$2 != "total" { s += $1 } END { print s + 0 }')

echo "migrations data: $size bytes (warn above $warn, fail above $limit)"
if [ "$size" -gt "$limit" ]; then
  echo "::error::migrations are $size bytes, over the 1 MiB ConfigMap limit. Squash them into a new baseline."
  exit 1
elif [ "$size" -gt "$warn" ]; then
  echo "::warning::migrations are $size bytes, over 768 KiB. Plan a baseline squash before they reach 1 MiB."
fi
