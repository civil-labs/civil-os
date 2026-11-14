#!/bin/sh
# PreToolUse hook: block Edit/Write/MultiEdit on generated code.
# Generated code is rebuilt from api/proto with `mise run generate`.
input=$(cat)
path=$(printf '%s' "$input" | sed -n 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
case "$path" in
  */api/gen/*|api/gen/*|*/internal/gen/*|internal/gen/*|*/sdk/js/gen/*|sdk/js/gen/*)
    echo "Blocked: $path is generated code. Edit the .proto under api/proto and run \`mise run generate\` instead." >&2
    exit 2
    ;;
esac
exit 0
