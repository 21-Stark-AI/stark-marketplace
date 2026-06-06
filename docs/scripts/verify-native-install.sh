#!/usr/bin/env bash
set -euo pipefail

# Verifies the native CC install loop offline:
#  1. marketplace.json is drift-free vs a fresh build
#  2. root uses owner, entries use author
#  3. every entry's string source resolves to a committed dist/claude/<bundle> tree
#
# Run from anywhere inside the repo.

repo_root="$(git rev-parse --show-toplevel)"
# The manifest lives at the repo root, so the marketplace root == repo root and
# relative entry sources resolve against $repo_root (CC's resolution rule).
manifest="$repo_root/.claude-plugin/marketplace.json"
bin="$(mktemp -t stark.XXXXXX)"
trap 'rm -f "$bin"' EXIT

echo "==> build stark"
( cd "$repo_root/engine" && go build -o "$bin" ./cmd/stark )

echo "==> drift check"
# stark resolves catalog/ + generated paths relative to its cwd, so run at repo root.
( cd "$repo_root" && "$bin" build --check )
echo "    drift-clean"

echo "==> manifest contract"
test -f "$manifest" || { echo "missing $manifest"; exit 1; }

# Root must carry owner, never author.
jq -e '.owner.name' "$manifest" >/dev/null || { echo "root missing owner.name"; exit 1; }
if jq -e 'has("author")' "$manifest" | grep -q true; then
  echo "root must NOT carry author"; exit 1
fi

# Every entry: author present, owner absent, source resolves.
count="$(jq '.plugins | length' "$manifest")"
echo "    $count plugin entr(y/ies)"
# Guard the loop: on macOS/BSD `seq 0 -1` prints "0\n-1" (not an empty range).
[ "$count" -gt 0 ] || { echo "    (no plugin entries)"; echo "==> OK: native install contract verified"; exit 0; }
for i in $(seq 0 $((count - 1))); do
  name="$(jq -r ".plugins[$i].name" "$manifest")"
  jq -e ".plugins[$i].author.name" "$manifest" >/dev/null \
    || { echo "entry $name missing author.name"; exit 1; }
  jq -e ".plugins[$i].version" "$manifest" >/dev/null \
    || { echo "entry $name missing version"; exit 1; }
  if jq -e ".plugins[$i] | has(\"owner\")" "$manifest" | grep -q true; then
    echo "entry $name must NOT carry owner"; exit 1
  fi
  src="$(jq -r ".plugins[$i].source" "$manifest")"
  if [[ "$src" == ./* ]]; then
    tree="$repo_root/${src#./}"
    test -d "$tree" || { echo "entry $name source tree missing: $tree"; exit 1; }
    echo "    $name -> $src (resolved)"
  else
    echo "    $name -> object source (skipping local resolve)"
  fi
done

echo "==> OK: native install contract verified"
