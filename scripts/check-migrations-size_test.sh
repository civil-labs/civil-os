#!/bin/sh
# Table tests for check-migrations-size.sh. Run from anywhere:
#   sh scripts/check-migrations-size_test.sh
# The migrations release workflow runs it before trusting the script.
set -eu

dir=$(dirname "$0")
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
failed=0

# check NAME WANT_EXIT WANT_LEVEL SIZE...
# Builds a directory with one file per SIZE (bytes), runs the script on it,
# and checks the exit code and which annotation (none, warning, error) it printed.
check() {
  name=$1 want_exit=$2 want_level=$3
  shift 3
  d="$tmp/$name"
  mkdir "$d"
  i=0
  for size in "$@"; do
    i=$((i + 1))
    head -c "$size" /dev/zero > "$d/$i.sql"
  done
  got_exit=0
  out=$(sh "$dir/check-migrations-size.sh" "$d") || got_exit=$?
  case $out in
    *::error::*) got_level=error ;;
    *::warning::*) got_level=warning ;;
    *) got_level=none ;;
  esac
  if [ "$got_exit" != "$want_exit" ] || [ "$got_level" != "$want_level" ]; then
    echo "FAIL $name: got exit $got_exit/$got_level, want $want_exit/$want_level"
    failed=1
  fi
}

check empty      0 none
check today      0 none    135352 40053 182   # current migrations
check at-warn    0 none    786432             # exactly 768 KiB
check warn-700k  0 none    716800             # 700 KiB
check warn-800k  0 warning 819200             # 800 KiB
check at-limit   0 warning 1048576            # exactly 1 MiB
check over-1_1m  1 error   1153434            # 1.1 MiB
check split      1 error   600000 500000      # sum over 1 MiB

# A missing directory is an error, not "0 bytes".
got_exit=0
sh "$dir/check-migrations-size.sh" "$tmp/missing" >/dev/null || got_exit=$?
if [ "$got_exit" != 1 ]; then
  echo "FAIL missing dir: got exit $got_exit, want 1"
  failed=1
fi

[ "$failed" -eq 0 ] && echo "ok"
exit "$failed"
