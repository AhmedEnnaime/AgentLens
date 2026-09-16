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
UPDATE event SET data = replace(data, '/Users/ahmedennaime', '/Users/user');
UPDATE project_directory SET directory = replace(directory, '/Users/ahmedennaime', '/Users/user');
UPDATE session SET title = replace(title, 'ahmedennaime', 'user');
UPDATE message SET data = replace(data, 'ahmedennaime', 'user');
UPDATE part SET data = replace(data, 'ahmedennaime', 'user');
UPDATE event SET data = replace(data, 'ahmedennaime', 'user');
UPDATE project_directory SET directory = replace(directory, 'ahmedennaime', 'user');
UPDATE message SET data = replace(data, 'AhmedEnnaime', 'GH-OWNER');
UPDATE part SET data = replace(data, 'AhmedEnnaime', 'GH-OWNER');
UPDATE event SET data = replace(data, 'AhmedEnnaime', 'GH-OWNER');
UPDATE session SET directory = replace(directory, 'AhmedEnnaime', 'GH-OWNER');
UPDATE project SET worktree = replace(worktree, 'AhmedEnnaime', 'GH-OWNER');
UPDATE project_directory SET directory = replace(directory, 'AhmedEnnaime', 'GH-OWNER');
UPDATE message SET data = json_set(data, '$.summary.diffs', '[TRUNCATED-FIXTURE-ONLY]') WHERE length(coalesce(json_extract(data, '$.summary.diffs'), '')) > 50000;
UPDATE part SET data = json_set(data, '$.state.output', '[TRUNCATED-FIXTURE-ONLY]') WHERE json_extract(data, '$.type') = 'tool' AND length(coalesce(json_extract(data, '$.state.output'), '')) > 50000;
UPDATE project SET name = CASE id WHEN 'c457eda94b7ffba3e9a7ddf1ff732c2f50ced7b4' THEN 'project-alpha' ELSE 'project-beta' END WHERE id IN ('c457eda94b7ffba3e9a7ddf1ff732c2f50ced7b4','e62da5f48a92c1290d4b4cb9b550549333c0d694');
SQL

python3 - "$tmp/snapshot.db" << 'PYEOF'
import re, sqlite3, sys
conn = sqlite3.connect(sys.argv[1])
pattern = re.compile(r'(?<![\w@.-])[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.(?:com|net|org|io|ma|dev|co|me)(?![\w.-])')
def scrub(text):
    return pattern.sub('EMAIL-REDACTED', text)
for table in ('message', 'part', 'event'):
    for (rowid, data) in list(conn.execute(f'SELECT rowid, data FROM {table} WHERE data LIKE "%@%"')):
        new = scrub(data)
        if new != data:
            conn.execute(f'UPDATE {table} SET data = ? WHERE rowid = ?', (new, rowid))
conn.commit()
conn.close()
PYEOF

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

  if [ "$include_children" = "yes" ]; then
    sqlite3 "$db" << SQL
DELETE FROM session WHERE id != '$session_id' AND (parent_id IS NULL OR parent_id != '$session_id');
SQL
  else
    sqlite3 "$db" << SQL
DELETE FROM session WHERE id != '$session_id';
SQL
  fi

  sqlite3 "$db" << SQL
DELETE FROM message WHERE session_id NOT IN (SELECT id FROM session);
DELETE FROM part WHERE session_id NOT IN (SELECT id FROM session);
DELETE FROM part WHERE message_id NOT IN (SELECT id FROM message);
DELETE FROM event WHERE aggregate_id NOT IN (SELECT id FROM session);
DELETE FROM event_sequence WHERE aggregate_id NOT IN (SELECT id FROM session);
DELETE FROM event WHERE id NOT IN (SELECT id FROM event ORDER BY seq LIMIT 10);
DELETE FROM todo WHERE session_id NOT IN (SELECT id FROM session);
DELETE FROM session_context_epoch WHERE session_id NOT IN (SELECT id FROM session);
DELETE FROM project WHERE id NOT IN (SELECT DISTINCT project_id FROM session);
DELETE FROM project_directory WHERE project_id NOT IN (SELECT id FROM project);
DELETE FROM workspace WHERE id NOT IN (SELECT DISTINCT workspace_id FROM session WHERE workspace_id IS NOT NULL);
SQL

  sqlite3 "$db" "PRAGMA wal_checkpoint(TRUNCATE);" > /dev/null
  rm -f "$db-wal" "$db-shm"
  local compact="$tmp/$name-compact.db"
  rm -f "$compact"
  sqlite3 "$db" "VACUUM INTO '$compact';"
  rm -f "$db"
  mv "$compact" "$out_dir/fixture.db"

  local sessions rows_msgs rows_parts child_sessions
  sessions=$(sqlite3 "$out_dir/fixture.db" "SELECT COUNT(*) FROM session;")
  child_sessions=$(sqlite3 "$out_dir/fixture.db" "SELECT COUNT(*) FROM session WHERE parent_id IS NOT NULL;")
  rows_msgs=$(sqlite3 "$out_dir/fixture.db" "SELECT COUNT(*) FROM message;")
  rows_parts=$(sqlite3 "$out_dir/fixture.db" "SELECT COUNT(*) FROM part;")

  local include_children_json="false"
  if [ "$include_children" = "yes" ]; then
    include_children_json="true"
  fi

  cat > "$out_dir/manifest.json" << EOF
{
  "name": "$name",
  "source_agent": "opencode",
  "source_version": "1.18.30",
  "session_root": "$session_id",
  "includes_subagent_children": $include_children_json,
  "rows": {"sessions": $sessions, "child_sessions": $child_sessions, "messages": $rows_msgs, "parts": $rows_parts},
  "notes": "event table trimmed to 10 sample rows (projection, not source of truth); session ids preserved for referential integrity; dangling parent_id in tool-heavy fixture is by design (its parent session lives in subagent-tree fixture)",
  "anonymization": ["usernames in paths -> /Users/user", "github owner name -> GH-OWNER", "project names -> placeholders", "known emails -> EMAIL-REDACTED", "oversized summary.diffs and tool outputs > 50KB truncated"],
  "provenance": "captured via sqlite3 .backup from live install $(date -u +%Y-%m-%d)"
}
EOF

  echo "$name: $sessions sessions ($child_sessions children), $rows_msgs messages, $rows_parts parts"
}

verify_fixture() {
  local db="$1"
  local leaks
  leaks=$(sqlite3 "$db" "SELECT (SELECT COUNT(*) FROM session WHERE directory LIKE '%ahmedennaime%' OR title LIKE '%ahmedennaime%') + (SELECT COUNT(*) FROM message WHERE data LIKE '%ahmedennaime%') + (SELECT COUNT(*) FROM part WHERE data LIKE '%ahmedennaime%') + (SELECT COUNT(*) FROM event WHERE data LIKE '%ahmedennaime%') + (SELECT COUNT(*) FROM project_directory WHERE directory LIKE '%ahmedennaime%') + (SELECT COUNT(*) FROM project WHERE worktree LIKE '%ahmedennaime%');")
  if [ "$leaks" -ne 0 ]; then
    echo "LEAK FAILURE in $db: $leaks rows still contain the username" >&2
    exit 1
  fi
  local emails
  emails=$(python3 - "$db" << 'PYEOF'
import re, sqlite3, sys
conn = sqlite3.connect(sys.argv[1])
pattern = re.compile(r'(?<![\w@.-])[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.(?:com|net|org|io|ma|dev|co|me)(?![\w.-])')
hits = 0
for table in ('message', 'part', 'event'):
    for (data,) in conn.execute(f'SELECT data FROM {table}'):
        if data and pattern.search(data):
            hits += 1
print(hits)
PYEOF
  )
  if [ "$emails" -ne 0 ]; then
    echo "LEAK FAILURE in $db: $emails rows contain email addresses" >&2
    exit 1
  fi
  local fk_violations
  fk_violations=$(sqlite3 "$db" "PRAGMA foreign_key_check;" | wc -l | tr -d ' ')
  if [ "$fk_violations" -ne 0 ]; then
    echo "FK FAILURE in $db: $fk_violations violations" >&2
    exit 1
  fi
}

extract "single-turn" "ses_f6fa95470ffepgysMr8gNzM72z" "no"
extract "multi-turn-build" "ses_f638a54dcffexgSgRLz7yzc27q" "no"
extract "subagent-tree" "ses_f7277fec8ffeousSUu48TJvWbl" "yes"
extract "tool-heavy" "ses_f6810faa4ffexF46JcRVUWI1Pu" "no"
extract "cache-heavy" "ses_f6fa9ab45ffeSbX2cC5s0kxT60" "no"

for f in single-turn multi-turn-build subagent-tree tool-heavy cache-heavy; do
  verify_fixture "$out_root/$f/fixture.db"
done

echo "all fixtures verified: zero leaks, zero FK violations"
echo "fixtures written to $out_root"