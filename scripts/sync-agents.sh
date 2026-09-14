#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
src="$repo_root/agents"
opencode_dir="$repo_root/.opencode/agent"
claude_dir="$repo_root/.claude/agents"

if [ ! -d "$src" ]; then
  echo "agents/ not found at $src" >&2
  exit 1
fi

mkdir -p "$opencode_dir" "$claude_dir"

find "$opencode_dir" -name '*.md' -type f ! -name 'README.md' -delete
find "$claude_dir" -name '*.md' -type f ! -name 'README.md' -delete

copied=0
for file in "$src"/*.md; do
  [ -e "$file" ] || continue
  name="$(basename "$file" .md)"
  cp "$file" "$opencode_dir/$name.md"
  cp "$file" "$claude_dir/$name.md"
  copied=$((copied + 1))
done

cp "$repo_root/AGENTS.md" "$repo_root/CLAUDE.md"

echo "synced $copied agents -> .opencode/agent/ and .claude/agents/"
echo "synced AGENTS.md -> CLAUDE.md"