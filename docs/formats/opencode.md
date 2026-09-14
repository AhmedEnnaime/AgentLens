# OpenCode Session Format Dossier

Reverse-engineered from a live OpenCode installation (v1.18.30 observed in session data). Every claim cites evidence in `testdata/fixtures/opencode/` or in the queries listed per section. Format Investigator deliverable for issue #4; feeds the OpenCode adapter (issues #8, #9, #10) and ADR-1 (issue #5).

**Status:** dossier v1 — covers everything the adapter needs for v0.1. Unknowns are explicitly recorded as unknown.

---

## 1. Storage layout

OpenCode stores all local state in a **SQLite database** — not JSONL files:

```
~/.local/share/opencode/
  opencode.db          ← the session store (SQLite, WAL mode)
  opencode.db-wal
  opencode.db-shm
  auth.json            ← credentials (NEVER read by AgentLens)
  log/                 ← app logs
  snapshot/            ← file-snapshot workspace copies (per project hash)
  tool-output/
  repos/
```

Relevant tables (`.tables`):

| Table | Role | Notes |
|---|---|---|
| `session` | One row per session (incl. subagent sessions) | Has `parent_id` → subagent trees; token/cost rollups; agent/model as JSON blobs |
| `message` | One row per user/assistant message | Full record is a JSON blob in `data` |
| `part` | Message parts (text, tool, reasoning, patch, compaction, step-*) | JSON blob in `data`; FK to message+session |
| `event` | Append-only projection of state changes (`session.created.1`, `message.updated.1`, `message.part.updated.1`, `session.updated.1`) | **Not the source of truth** — `session`/`message`/`part` tables are. Event table exists for incremental sync/streaming |
| `project` / `workspace` / `project_directory` | Project registry (worktree, VCS, name) | Sessions FK to `project_id` |
| `todo` | Per-session todo lists (content, status, priority, position) | Observable task markers! |
| `session_context_epoch` | Compaction baseline bookkeeping | `baseline`, `snapshot`, `baseline_seq` |
| `migration` / `event_sequence` / `data_migration` | Infrastructure | Ignore |

**Evidence:** `sqlite3 ~/.local/share/opencode/opencode.db ".tables"` / `.schema <table>` — 44 sessions, 1,511 messages, 5,971 parts, 21,478 events, 3 projects in the investigated install.

**Provenance note:** timestamps are epoch **milliseconds** throughout.

---

## 2. Session record

`session` table columns (schema is the contract; JSON-blob columns noted):

| Field | Type | Meaning | Provenance for us |
|---|---|---|---|
| `id` | text | `ses_<random>` | observed |
| `project_id` | text | FK to project | observed |
| `parent_id` | text | **set for subagent sessions** — points to the parent session row | observed |
| `slug` | text | human-ish name (`sunny-garden`) | observed |
| `directory` | text | working dir at session start | observed |
| `title` | text | LLM-generated title (e.g. "Fix golangci-lint CI pin (@implementer subagent)") | observed (generated, but stored) |
| `version` | text | OpenCode version that created it (`1.18.30`) | observed |
| `cost` | real | session cost rollup — **observed 0.0 everywhere** (provider not reporting cost) | observed-as-zero → we treat provider cost as **unavailable**, not zero |
| `tokens_input/output/reasoning/cache_read/cache_write` | int | session rollups | observed (rollup, derived by OpenCode — label accordingly) |
| `summary_additions/deletions/files` | int | diff summary | observed |
| `summary_diffs` | text | diff text | observed |
| `metadata` | text | JSON | unknown (empty in observed data) |
| `revert`, `permission` | text | | unknown (empty in observed data) |
| `agent` | text | **JSON blob**: `{"id":"glm-5.3","providerID":"ollama-cloud","variant":"max"}` — default agent model config | observed |
| `model` | text | | unknown (empty in observed data) |
| `time_created/time_updated` | int | epoch ms | observed |
| `time_compacting` | int | set when compaction happened | observed |
| `time_archived` | int | | observed when set |

---

## 3. Message record

`message.data` JSON — **the richest record; this is where per-call usage lives.**

### 3.1 User message

```json
{
  "role": "user",
  "time": { "created": 1789080895833 },
  "agent": "build",
  "model": { "providerID": "ollama", "modelID": "glm-5.3:cloud" },
  "summary": { "diffs": [] }
}
```

### 3.2 Assistant message

```json
{
  "parentID": "msg_08d880159001DRYWH5Pc0U3wV0",
  "role": "assistant",
  "mode": "build",
  "agent": "build",
  "path": { "cwd": "/Users/.../MatchZone", "root": "/Users/.../MatchZone" },
  "cost": 0,
  "tokens": {
    "total": 9831,
    "input": 9313,
    "output": 390,
    "reasoning": 0,
    "cache": { "write": 0, "read": 128 }
  },
  "modelID": "glm-5.3:cloud",
  "providerID": "ollama",
  "time": { "created": 1789080895854, "completed": 1789080901581 },
  "finish": "tool-calls"
}
```

Field-by-field:

| Field | Meaning | Provenance |
|---|---|---|
| `parentID` | The user message (turn) this assistant response belongs to → **turn boundaries are observed, not inferred** | observed |
| `role` | `user` / `assistant` | observed |
| `agent` | Agent that produced it: observed values `build`, `explore`, `reviewer`, `implementer`, `architect`, `compaction`, `compat-probe` | observed |
| `mode` | Mode active for the message (`build`) | observed |
| `path.cwd` / `path.root` | Working directory | observed |
| `cost` | Per-message cost — **0 in all observed data** | observed-as-zero → provider cost **unavailable** |
| `tokens.input/output/reasoning` | Per-message usage | observed |
| `tokens.cache.read/write` | Cache hits/writes | observed |
| `modelID` / `providerID` | Per-call model identity (flat fields, not nested) | observed |
| `time.created` / `time.completed` | Per-call latency = completed − created | observed |
| `finish` | `tool-calls`, (others: `stop`, `length`… — only `tool-calls` observed) | observed |

**Quirk:** user messages carry the model config *requested at prompt time* (nested `model` object); assistant messages carry *what actually ran* (flat `modelID`/`providerID`). **Always use the assistant record for attribution.**

**Coverage check (this install):** 1,395 of 1,404 assistant messages have `time.completed` → 99.4% timing completeness. 0 of N have nonzero cost.

---

## 4. Part records

`part.data` JSON, ordered by (`message_id`, `id` insertion). Observed types with counts:

| `type` | Count | Meaning | Key fields |
|---|---|---|---|
| `tool` | 1,597 | Tool call + result | `tool`, `callID`, `state` |
| `step-start` | 1,403 | Agent-loop step boundary | `snapshot` (workspace hash) |
| `step-finish` | 1,383 | Step end + **per-step usage** | `reason`, `snapshot`, `tokens`, `cost` |
| `text` | 776 | Assistant/user text content | `text` |
| `reasoning` | 572 | Thinking tokens content | `text`, `time.start/end` |
| `patch` | 244 | File edit applied | `hash`, `files[]` |
| `compaction` | 6 | Context compaction event | `auto`, `tail_start_id` |

### 4.1 Tool part (the full shape)

```json
{
  "type": "tool",
  "tool": "webfetch",
  "callID": "call_ee1gzj1v",
  "state": {
    "status": "completed",
    "input": { "format": "markdown", "url": "https://..." },
    "output": "ChatGPT - Ollama Coding Workflow",
    "metadata": { "truncated": false },
    "title": "https://... (text/html; charset=utf-8)",
    "time": { "start": 1789080900192, "end": 1789080901470 }
  }
}
```

- `state.status`: `completed` / `error` / (`running` transient)
- `state.input`: tool arguments (varies per tool — treat as opaque JSON)
- `state.output`: tool result text (varies; sometimes JSON string)
- `state.error`: error message when status=error (observed: edit-tool oldString mismatch)
- `state.time.start/end`: **per-tool-call duration observed**
- Edit-tool `input.filePath`, `input.oldString/newString`: file-change attribution (huge; treat as content — reference only per D28)

### 4.2 Patch part

```json
{ "type": "patch", "hash": "cea02a...", "files": ["/abs/path/file.go"] }
```

→ observed file-change list; the `hash` matches `snapshot` values in step-start/step-finish.

### 4.3 Step parts (the agent loop)

- `step-start.snapshot` = workspace hash at step start
- `step-finish`: `reason` (`tool-calls`/…), `tokens` (same shape as message tokens), `cost` (0 observed)
- step-finish ≈ **one model call within a turn** — this is the per-call usage record; message-level tokens are the turn rollup

### 4.4 Compaction part

```json
{ "type": "compaction", "auto": true, "tail_start_id": "msg_0905..." }
```

→ compaction events observed directly (context-pressure signal for v0.2).

---

## 5. Subagent structure

- Subagent runs are **child sessions**: `session.parent_id` → parent session id.
- Observed tree: one MatchZone session (`ses_f7277...`) with **26 child sessions** — titles like `"Fix golangci-lint CI pin (@implementer subagent)"`, agents `implementer`/`reviewer`, models `glm-5.3-flash:cloud`, `nemotron-3-ultra-free`.
- The agent identity of a subagent session is visible in: session `title` suffix (`(@implementer subagent)`), messages' `agent` field, and the session's `agent` JSON blob (default model config).
- **Turns inside a subagent session are the same user/assistant structure** — the "user" messages in a subagent session are the orchestrator's task prompt(s).

**Provenance:** the parent→child hierarchy is **observed** (parent_id), not inferred. Task decomposition beyond subagent boundaries remains inferred (v0.2 scope, unchanged).

---

## 6. Todos (observed task markers)

`todo` table: per-session ordered list (`content`, `status`, `priority`, `position`, `time_created/updated`). These are the plan-tool records → **observed task boundaries** for per-task attribution (MVP best-effort scope).

---

## 7. Model identity across the install

Observed (modelID, providerID) pairs: `glm-5.3:cloud`/`ollama`, `glm-5.3-flash:cloud`/`ollama`, `glm-5.3`/`ollama-cloud`, `big-pickle`/`opencode` (free-tier proxy), `nemotron-3-ultra-free`/`opencode`, `ling-3.0-flash-fin-free`/`opencode`.

→ Model catalog must key on `(providerID, modelID)` pairs; `variant` appears in session-level agent blobs (`"variant":"max"`).

---

## 8. Event table (projection, not truth)

`event.type` values observed: `session.created.1`, `session.updated.1`, `message.updated.1`, `message.part.updated.1`. Each row: `aggregate_id` (session id), `seq`, `data` (JSON with sessionID + payload).

**Adapter decision:** read `session`/`message`/`part` tables as source of truth (current state); the `event` table is OpenCode's streaming projection — useful later for live watch (post-v1.0), not needed for v0.1 post-hoc analysis.

---

## 9. Quirks & unknowns (recorded honestly)

1. `cost` is 0 in **every** message/session observed → treat provider cost as `unavailable`; AgentLens computes `[estimated]` costs from usage × pricing (issue #11).
2. User vs assistant model fields differ in shape (nested vs flat) — see §3.
3. `session.model` column empty everywhere; the real default-model blob lives in `session.agent` JSON.
4. Reasoning parts have `text` content but **no separate reasoning-token count field** — `tokens.reasoning` exists at message level; per-part reasoning time is observed (`time.start/end`).
5. `part.state.output` can be very large (tool output) — content-reference policy (D28) applies; fixtures must truncate/scrub.
6. `snapshot` system (`~/.local/share/opencode/snapshot/<project-hash>/`) holds file workspace copies — AgentLens does not need it for v0.1 (edits are observable via parts/patches); noted as available.
7. `session_context_epoch.baseline/snapshot/baseline_seq` — compaction internals; exact semantics unknown (recorded, not needed for v0.1).
8. `auth.json` contains credentials — AgentLens must **never read it** (adapter hard rule).
9. WAL mode: reading the DB while OpenCode runs is safe for reads, but fixtures must be taken via `sqlite3 .backup` to get a consistent snapshot.
10. `finish` values other than `tool-calls` not observed in this install (expect `stop`, `length`); parser must default-accept unknown finishes.

---

## 10. Golden fixtures

`testdata/fixtures/opencode/` — each fixture is a **consistent SQLite snapshot** (`.backup` dump) + a manifest:

| Fixture | Source session | Shows |
|---|---|---|
| `single-turn/` | Short 1-turn session | Minimal user→assistant pair, one tool call |
| `multi-turn-build/` | Medium build session | Many turns, step-start/finish loop, patches |
| `subagent-tree/` | MatchZone orchestration session | Parent + child sessions, per-subagent models/agents |
| `tool-heavy/` | Session dense in tool calls | Edit/bash/read/write states incl. an error state |
| `cache-heavy/` | Session with high cache_read | Token cache attribution |

**Anonymization applied:** usernames in paths (`/Users/ahmedennaime` → `/Users/user`), session ids re-randomized per fixture, project names mapped to placeholders, all message/part text content truncated to 0 (v0.1 fixtures carry structure + usage only, not content — content re-read policy D28; content-bearing fixtures arrive with issue #13 redaction tests), secrets scrubbed by inspection.

**Manifest format** (`manifest.json` per fixture): source (session id pattern + OpenCode version), rows (sessions/messages/parts), anonymization steps applied, known quirks present, checksum.

---

## 11. What the adapter gets from this dossier (mapping preview)

| AgentLens need | OpenCode source | Provenance |
|---|---|---|
| Session identity | `session.id`, `title`, `directory`, `project_id` | observed |
| Turns | user messages; assistant `parentID` | observed |
| Per-model-call usage & timing | assistant messages (`tokens`, `time`) + `step-finish` parts | observed |
| Agent identity | message `agent`; subagent `session.parent_id` + titles | observed |
| Model identity | assistant `modelID`/`providerID` | observed |
| Tool calls + durations | `part.type=tool` (`state.time`, `state.status`) | observed |
| File edits | `part.type=patch` (`files`); edit-tool inputs (reference only) | observed |
| Tests/lint outcomes | bash-tool parts with `state.output` (parse in v0.2 #smells; content referenced, not stored) | derived (from observed output) |
| Compactions | `part.type=compaction`; `session.time_compacting` | observed |
| Task markers | `todo` table | observed |
| Provider cost | `cost` fields — all zero | **unavailable** → estimate via pricing |
| Retry loops | tool-error parts + repeated step-finish reasons | derived (v0.2) |

---

## 12. Open questions for the Architect (ADR-1 input)

1. Should the canonical model treat `step-finish` as the model-call span and message-level tokens as a rollup, or both as calls? (Recommendation: step = call; message = turn rollup — matches §4.3 semantics.)
2. Session-level `agent` blob carries `variant` (`max`) — keep variant as a model-identity attribute? (Recommendation: yes, as an attribute.)
3. Subagent "task prompt" arrives as a user message in the child session — the dossier records this; the task-inference view (v0.2) should treat child sessions as one observed task unit each. Confirm in ADR-1.