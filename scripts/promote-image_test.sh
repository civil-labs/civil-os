#!/bin/sh
# Table tests for promote-image.sh, against a fake imagetools. Run from
# anywhere:
#   sh scripts/promote-image_test.sh
# The release workflow runs it before trusting the script
# (docs/image-promotion/plan.md, step 2).
set -eu

dir=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
failed=0

# The fake registry: $work/reg/<ref with / and : as _> holds a digest.
# $work/appear-after/<ref> holds a number of polls before that ref exists.
# $work/state/mismatch makes create store a different digest. Every call is
# logged to $work/state/log.
cat > "$work/imagetools" <<'EOF'
#!/bin/sh
set -eu
state=$FAKE_STATE
key() { printf '%s' "$1" | tr '/:' '__'; }
echo "$*" >> "$state/log"
case $1 in
  inspect)
    k=$(key "$2")
    if [ -f "$state/appear-after/$k" ]; then
      n=$(cat "$state/appear-after/$k")
      if [ "$n" -gt 0 ]; then
        echo $((n - 1)) > "$state/appear-after/$k"
        echo "ERROR: $2: not found" >&2
        exit 1
      fi
    fi
    if [ -f "$state/reg/$k" ]; then cat "$state/reg/$k"; exit 0; fi
    echo "ERROR: $2: not found" >&2
    exit 1
    ;;
  create)
    # create --tag TARGET SOURCE
    src=$(cat "$state/reg/$(key "$4")")
    if [ -f "$state/mismatch" ]; then src=sha256:different; fi
    echo "$src" > "$state/reg/$(key "$3")"
    ;;
esac
EOF

image=ghcr.io/example/svc
sha=0123456789abcdef0123456789abcdef01234567
src_ref=ghcr.io_example_svc_sha-0123456
rel_ref=ghcr.io_example_svc_v1.2.3
digest=sha256:aaaa

# reset: a fresh fake registry for one case.
reset() {
  rm -rf "$work/state"
  mkdir -p "$work/state/reg" "$work/state/appear-after"
  : > "$work/state/log"
}

# run EXPECTED_EXIT ARGS...: run the script, keep stdout/stderr, check the exit.
run() {
  want=$1
  shift
  got=0
  FAKE_STATE=$work/state IMAGETOOLS="sh $work/imagetools" PROMOTE_INTERVAL=0 PROMOTE_TIMEOUT=5 \
    sh "$dir/promote-image.sh" "$@" > "$work/out" 2> "$work/err" || got=$?
  if [ "$got" != "$want" ]; then
    echo "FAIL [$case] promote-image.sh $*: exit $got, want $want; stderr: $(cat "$work/err")"
    failed=1
  fi
}

creates() { grep -c '^create' "$work/state/log" || true; }

expect() {
  if ! eval "$1"; then
    echo "FAIL [$case] expected: $1"
    failed=1
  fi
}

case="sha present, release absent"
reset
echo "$digest" > "$work/state/reg/$src_ref"
run 0 "$image" "$sha" v1.2.3
expect '[ "$(creates)" = 1 ]'
expect '[ "$(cat "$work/out")" = "| $image | v1.2.3 | $digest |" ]'
expect '[ "$(cat "$work/state/reg/$rel_ref")" = "$digest" ]'
expect 'grep -q "^inspect $image:sha-0123456 " "$work/state/log"'

case="sha appears after two polls"
reset
echo "$digest" > "$work/state/reg/$src_ref"
echo 2 > "$work/state/appear-after/$src_ref"
run 0 "$image" "$sha" v1.2.3
expect '[ "$(creates)" = 1 ]'

case="sha never appears"
reset
run 1 "$image" "$sha" v1.2.3
expect '[ "$(creates)" = 0 ]'
expect 'grep -q "sha-0123456 not found after 5s. Did the main Images run for $sha fail?" "$work/err"'

case="release exists, same digest"
reset
echo "$digest" > "$work/state/reg/$src_ref"
echo "$digest" > "$work/state/reg/$rel_ref"
run 0 "$image" "$sha" v1.2.3
expect '[ "$(creates)" = 0 ]'
expect '[ "$(cat "$work/out")" = "| $image | v1.2.3 | $digest |" ]'

case="release exists, different digest"
reset
echo "$digest" > "$work/state/reg/$src_ref"
echo sha256:bbbb > "$work/state/reg/$rel_ref"
run 1 "$image" "$sha" v1.2.3
expect '[ "$(creates)" = 0 ]'
expect '[ "$(cat "$work/state/reg/$rel_ref")" = sha256:bbbb ]'

case="create yields a different digest"
reset
echo "$digest" > "$work/state/reg/$src_ref"
touch "$work/state/mismatch"
run 1 "$image" "$sha" v1.2.3
expect 'grep -q "after promotion" "$work/err"'

case="registry error is not treated as missing"
reset
echo "$digest" > "$work/state/reg/$src_ref"
echo "$digest" > "$work/state/reg/$rel_ref"
# Looking up the release tag fails with an error other than "not found".
cat > "$work/imagetools-broken" <<EOF
#!/bin/sh
if [ "\$1" = inspect ] && [ "\$2" = "$image:v1.2.3" ]; then
  echo "ERROR: connection reset by peer" >&2; exit 1
fi
exec sh "$work/imagetools" "\$@"
EOF
got=0
FAKE_STATE=$work/state IMAGETOOLS="sh $work/imagetools-broken" PROMOTE_INTERVAL=0 PROMOTE_TIMEOUT=5 \
  sh "$dir/promote-image.sh" "$image" "$sha" v1.2.3 > "$work/out" 2> "$work/err" || got=$?
expect '[ "$got" = 1 ]'
expect '[ "$(creates)" = 0 ]'

for bad in "$image $sha" "$image 0123456 v1.2.3" "$image $sha vfoo" "$image $sha v1.2" "$image $sha v1.2.3-rc.1" "$image $(echo "$sha" | tr a-f A-F) v1.2.3"; do
  case="usage: $bad"
  reset
  # shellcheck disable=SC2086 # split the argument list on purpose
  run 2 $bad
  expect '[ "$(creates)" = 0 ]'
done

if [ "$failed" -ne 0 ]; then
  exit 1
fi
echo "promote-image.sh: all cases pass"
