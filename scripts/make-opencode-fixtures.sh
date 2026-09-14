#!/usr/bin/env bash
set -euo pipefail

src_db="$HOME/.local/share/opencode/opencode.db"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_root="$repo_root/testdata/fixtures/opencode"
tmp="/tmp/agentlens-fixture"

if [ ! -f "$src_db" ]; then
  echo "opencode.db not found at $src_db" >&2
  exit 1
fi

mkdir -p "$tmp" "$out_root"

rm -f "$tmp/snapshot.db"
sqlite3 "$src_db" ".backup $tmp/snapshot.db"

sqlite3 "$tmp/snapshot.db" << 'SQL'
UPDATE session SET directory = replace(directory, '/Users/ahmedennaime', '/Users/user');
UPDATE project SET worktree = replace(worktree, '/Users/ahmedennaime', '/Users/user');
UPDATE message SET data = replace(data, '/Users/ahmedennaime', '/Users/user');
UPDATE part SET data = replace(data, '/Users/ahmedennaime', '/Users/user');
UPDATE session SET title = replace(title, 'ahmedennaime', 'user');
UPDATE message SET data = replace(data, 'ahmedennaime', 'user');
UPDATE part SET data = replace(data, 'ahmedennaime', 'user');
UPDATE message SET data = replace(data, 'AhmedEnnaime', 'GH-OWNER');
UPDATE part SET data = replace(data, 'AhmedEnnaime', 'GH-OWNER');
UPDATE session SET directory = replace(directory, 'AhmedEnnaime', 'GH-OWNER');
UPDATE project SET worktree = replace(worktree, 'AhmedEnnaime', 'GH-OWNER');
UPDATE message SET data = json_set(data, '$.summary.diffs', '[TRUNCATED-FIXTURE-ONLY]') WHERE length(coalesce(json_extract(data, '$.summary.diffs'), '')) > 50000;
UPDATE part SET data = json_set(data, '$.state.output', '[TRUNCATED-FIXTURE-ONLY]') WHERE json_extract(data, '$.type') = 'tool' AND length(coalesce(json_extract(data, '$.state.output'), '')) > 50000;
UPDATE project SET name = CASE id WHEN 'c457eda94b7ffba3e9a7ddf1ff732c2f50ced7b4' THEN 'project-alpha' ELSE 'project-beta' END WHERE id IN ('c457eda94b7ffba3e9a7ddf1ff732c2f50ced7b4','e62da5f48a92c1290d4b4cb9b550549333c0d694');
SQL

extract() {
  local name="$1"
  local session_id="$2"
  local include_children="$3"
  local out_dir="$out_root/$name"
  rm -rf "$out_dir"
  mkdir -p "$out_dir"

  local db="$tmp/$name.db"
  rm -f "$db"
  sqlite3 "$tmp/snapshot.db" ".backup $db"

  local child_filter=""
  if [ "$include_children" = "yes" ]; then
    child_filter="OR session_id IN (SELECT id FROM session WHERE parent_id = '$session_id')"
  fi

  local event_child_filter=""
  if [ "$include_children" = "yes" ]; then
    event_child_filter="OR aggregate_id IN (SELECT id FROM session WHERE parent_id = '$session_id')"
  fi

  sqlite3 "$db" << SQL
DELETE FROM session WHERE id != '$session_id' AND (parent_id IS NULL OR parent_id != '$session_id');
DELETE FROM message WHERE session_id != '$session_id' $child_filter;
DELETE FROM part WHERE session_id != '$session_id' $child_filter;
DELETE FROM event WHERE aggregate_id != '$session_id' AND aggregate_id NOT IN (SELECT id FROM session WHERE parent_id = '$session_id');
DELETE FROM event WHERE id NOT IN (SELECT id FROM event ORDER BY seq LIMIT 10);
DELETE FROM event_sequence WHERE aggregate_id != '$session_id' $event_child_filter;
DELETE FROM todo WHERE session_id != '$session_id' $child_filter;
DELETE FROM session_context_epoch WHERE session_id != '$session_id' $child_filter;
DELETE FROM project WHERE id NOT IN (SELECT DISTINCT project_id FROM session);
DELETE FROM workspace WHERE id NOT IN (SELECT DISTINCT workspace_id FROM session WHERE workspace_id IS NOT NULL);
SQL

  sqlite3 "$db" "PRAGMA wal_checkpoint(TRUNCATE);" > /dev/null
  rm -f "$db-wal" "$db-shm"
  local compact="$tmp/$name-compact.db"
  rm -f "$compact"
  sqlite3 "$db" "VACUUM INTO '$compact';"
  rm -f "$db"
  mv "$compact" "$out_dir/fixture.db"

  local sessions rows_msgs rows_parts
  sessions=$(sqlite3 "$out_dir/fixture.db" "SELECT COUNT(*) FROM session;")
  rows_msgs=$(sqlite3 "$out_dir/fixture.db" "SELECT COUNT(*) FROM message;")
  rows_parts=$(sqlite3 "$out_dir/fixture.db" "SELECT COUNT(*) FROM part;")

  cat > "$out_dir/manifest.json" << EOF
{
  "name": "$name",
  "source_agent": "opencode",
  "source_version": "1.18.30",
  "session_root": "$session_id",
  "includes_subagent_children": $include_children,
  "rows": {"sessions": $sessions, "messages": $rows_msgs, "parts": $rows_parts},
  "anonymization": ["username in paths -> /Users/user", "project names -> placeholders", "session ids preserved for referential integrity"],
  "content_policy": "structure + usage only; full text content remains in fixture for offline parser testing",
  "provenance": "captured via sqlite3 .backup from live install $(date -u +%Y-%m-%d)"
}
EOF

  echo "$name: $sessions sessions, $rows_msgs messages, $rows_parts parts"
}

extract "single-turn" "ses_f6fa95470ffepgysMr8gNzM72z" "no"
extract "multi-turn-build" "ses_f638a54dcffexgSgRLz7yzc27q" "no"
extract "subagent-tree" "ses_f7277fec8ffeousSUu48TJvWbl" "yes"
extract "tool-heavy" "ses_f6810faa4ffexF46JcRVUWI1Pu" "no"
extract "cache-heavy" "ses_f6fa9ab45ffeSbX2cC5s0kxT60" "no"

echo "fixtures written to $out_root"