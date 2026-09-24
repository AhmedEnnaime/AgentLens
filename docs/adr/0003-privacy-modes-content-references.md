# ADR-3: Privacy Modes & Content References

**Status:** Accepted (owner-approved 2026-09-15 with the mini-plan for issue #7; all flags approved)
**Architect:** Architect agent (`ollama-cloud/glm-5.3`) · **Issue:** #7 · **Inputs:** ADR-1 (canonical model), ADR-2 (storage schema), OpenCode dossier v1 (issue #4), fixtures in `testdata/fixtures/opencode/`

## 1. Context

v0.1's privacy posture is a hard product promise (AGENTS.md §3: no full prompt/response content in AgentLens storage by default; references + on-demand re-read only). D28 of the decision log fixed that posture, but three unresolved tensions force this ADR rather than an issue comment:

1. **Where the content reference points.** ADR-1 §2 records tool input/output as "ref to raw event". In the default metadata-only mode the raw event itself is stripped, so the reference must point at the *agent's source* (the OpenCode SQLite DB), not at our evidence layer. This supersedes that ADR-1 row.
2. **The mode→storage contract.** The storage layer (ADR-2) stores raw payloads verbatim-by-contract, so the strip must happen *before* ingest, and its fail-closed behavior (what is content in an unknown shape) must be a declared contract, not an adapter convenience.
3. **Graceful degradation as a contract.** Re-read of content depends on the source DB continuing to exist; its absence or mutation must degrade to typed statuses, not error-message conventions.

## 2. Decision

### D1 — Content reference

A small immutable domain value: *"the full content of X lives at source Y, keyed Z, and hashed H at capture."*

```go
type ContentKind uint8 // v0.1: ContentKindSQLiteRow only

type ContentRef struct {
    Kind   ContentKind `json:"kind"`    // "sqlite-row"
    Path   string      `json:"path"`    // absolute path of the source artifact (the agent's DB)
    Table  string      `json:"table"`   // "part" | "message" | "todo" | "session"
    RowID  string      `json:"row_id"`  // native row id (part id, message id, …)
    SHA256 string      `json:"sha256"`  // SHA-256 over the value's raw JSON bytes (byte-exact)
    Bytes  int64       `json:"bytes"`   // raw byte length at capture
}

func (r ContentRef) Validate() error
func AsContentRef(v any) (ContentRef, error)
```

`sqlite-row` with generic `Table`/`RowID` fields is deliberately **not** agent-specific — the shape "sqlite table + row id" is universal; only the *values* are OpenCode's. Adapter isolation holds. `Validate` requires a known kind, non-empty `Path`/`Table`/`RowID`, and `SHA256` = 64 lowercase hex.

### D2 — Two layers, one wire shape

(i) **In raw payloads, in place**: the strip replaces the content value *under the same key* with the ref object (`"text": {"agentlens.content": {…ref…}}`). This is mandatory, not cosmetic: everything derived is rebuildable from raw events, so refs must be recoverable from the evidence layer — if refs lived only on spans, re-normalization could not reproduce them once the text is stripped. (ii) **On span/event attributes** (`agentlens.content.input`, `agentlens.content.output`, `agentlens.content.text`) — vocabulary fixed here so #9 applies it; actual attachment is #9's normalize work.

### D3 — Two distinct pointer concepts, never conflated

`Span.SourceRef` = **evidence lineage** (which raw record produced this span — provenance). `ContentRef` = **D28 re-read key** (where the full text lives at the source). In metadata-only the raw event is stripped, so evidence lineage survives but re-read must go to the source. This is why ADR-1's "ref to raw event" for content is superseded by this ADR.

### D4 — Mode boundary: the adapter owns the strip; the strip is a pure function

Only the adapter knows which agent-native fields are content (`part.data.text`, `part.data.state.input/output/error`, todo `content`, session `summary_diffs`) — a normalize-layer or storage-side strip would smuggle OpenCode knowledge past the ACL. So:

```go
func StripContent(mode config.PrivacyMode, sourcePath, recordType string, payload []byte) (stripped []byte, refs []domain.ContentRef, err error)
```

- **metadata-only** (default): strip the content-field list, replace each with a ref (D2). Fail-closed on unknown `part.data.type` or unknown `recordType` — we cannot know what is content in a shape we've never seen, and a silent pass-through would be a silent privacy breach (Amendment-B posture: never persist an undefined distinction; privacy > availability on format drift).
- **content-local** (opt-in): payload passes through **verbatim** (byte-identical) but refs are extracted and returned; payload untouched (byte-exact fidelity for the evidence layer).
- **redacted-export**: `StripContent` rejects it (see D7).

### D5 — Content-field map (metadata-only strip list), from the dossier

| Record | Content fields stripped | Kept as metadata |
|---|---|---|
| `part` (text/reasoning) | `data.text` | everything else |
| `part` (tool) | `data.state.input`, `data.state.output`, `data.state.error` | `state.status`, `state.time`, `state.metadata`, `state.title`, `data.files` (paths), patch `hash`, `callID`, `tool` |
| `part` (patch) | — (no content) | `files[]`, `hash` |
| `todo` | `content` | `status`, `priority`, `position`, timestamps |
| `session` | `summary_diffs` | `title`, `slug`, `directory`, all usage/timing/identity fields |

Known part types (else fail-closed): `text`, `reasoning`, `tool`, `patch`, `compaction`, `step-start`, `step-finish`. Known record types (else fail-closed): `session`, `message`, `part`, `todo`.

**Hash rule.** `SHA256` is computed over the content value's **raw JSON bytes as they appear in the source payload** — extracted via `json.RawMessage`, never re-marshaled. Re-marshaling would assume map-key ordering determinism that `encoding/json` does not promise for map values; raw extraction is byte-exact by construction.

### D6 — Mode config: file, not flags

`~/.agentlens/config.json` (env override `AGENTLENS_CONFIG`, mirroring `AGENTLENS_DB`), new package `internal/config`:

```go
type PrivacyMode uint8 // PrivacyMetadataOnly | PrivacyContentLocal | PrivacyRedactedExport
func (m PrivacyMode) String() string   // "metadata-only" | "content-local" | "redacted-export"
type Config struct { Version int; PrivacyMode PrivacyMode }
func Default() Config                  // metadata-only
func Load(path string) (Config, error) // absent file → defaults (documented posture, not an error); present file must be fully valid
func (c Config) Validate() error       // version 1; known mode; unknown keys rejected (fail-loud)
func (m PrivacyMode) SupportsImport() bool // redacted-export → false until #14
```

Mode is a durable posture, not a per-command choice — no CLI flag in v0.1. Config-file absence = metadata-only default; a *present but invalid* file never silently downgrades to anything.

### D7 — `redacted-export` split

Declared vocabulary, parsed by config, **rejected at import** with a fail-loud "arrives with #14 (`agentlens share`)" — redaction is an egress concern; treating it as equivalent to metadata-only today would be an implicit mode change. Nothing consumes it in #7.

### D8 — Re-read API: domain-declared interface, adapter-implemented

```go
type ContentStatus uint8 // ContentResolved | ContentUnavailable | ContentChanged
type ResolvedContent struct { Status ContentStatus; Text string } // Text valid iff Resolved
type ContentResolver interface {
    ResolveContent(ctx context.Context, ref ContentRef) (ResolvedContent, error) // error = operational failure only
}
```

Semantics: `Resolved` = re-read now, hash matches (text is observed-at-read-time); `Unavailable` = source file moved/absent/unreadable/row gone → report renders with metadata, never breaks; `Changed` = source present but hash differs from capture → never silently displayed as if current. The three-status result (not `(string, error)`) is what makes graceful degradation a contract rather than an error-message convention. The exported `RunContentResolverContract` pins the bar that #8's real impl and #15's harness must both pass.

### D9 — D28 tension, stated honestly

The storage layer stores raw payloads verbatim-by-contract, therefore the strip must happen before ingest (D4): in metadata-only mode nothing content-y ever enters AgentLens's DB. The honest consequence: **AgentLens's DB then holds refs whose fulfillment depends on OpenCode's DB continuing to exist at its captured path.** Moved/absent → `Unavailable`; row content changed → `Changed`. That is the designed degradation, not a bug. A future snapshot mode is the only way out and is explicitly out of scope.

### D10 — Immutability × mode flips (consequence accepted)

Raw events are immutable (ADR-2 D8). Re-importing the same trace under a different mode produces different payload bytes for the same ids → `ErrConflict` (ADR-2 D9). Correct and fail-loud; but it means **there is no retroactive strip** — a store that ever imported content-local keeps those bytes until a future purge issue. Pre-release DBs are disposable (ADR-2 Amendment A precedent). Mixed-mode stores are possible because mode binds per-import; the trace header records the latest import's mode (last-import-wins is ADR-2's only header write).

### D11 — Import mode recorded on the trace

`SourceMetadata.PrivacyMode string` (`json:"privacy_mode,omitempty"`; plain string like `ImportedAt` — capture-process metadata, not session telemetry, so no `Value[T]` wrapping). Additive, wire-layer only; `""` = pre-mode-tracking import. Lets #12 render "content re-readable: yes/no" honestly. This is the only domain-struct change, and it is why this ADR exists.

### D12 — v0.1 re-read is source-backed only, in both modes

`ContentRef` always points at the agent's source; mode changes only payload retention. A raw-event-backed resolver (content-local fallback when the source is gone) is deferred — it needs a content→raw-event mapping that does not exist yet. Declared future work, not hidden scope.

## 3. Consequences

- **Supersession.** ADR-1 §2's "Tool input/output → ref to raw event" row is superseded: in metadata-only the raw event is stripped, so content references point at the agent's source, not the evidence layer. ADR-1's `agentlens.content.sha256`/`agentlens.content.bytes` attribute names remain valid (the ref carries the same identity fields), but the reference target is corrected by this ADR.
- **Zero DDL change.** Refs ride inside span/event attribute blobs and inside raw payload bytes; the only wire change is the additive `SourceMetadata.privacy_mode` field. ADR-2 is untouched.
- **No retroactive strip** (D10): a store that ever imported content-local keeps those bytes until a future purge issue. Pre-release DBs are disposable.
- **Re-read degradation is a contract** (D8/D9): `Unavailable` and `Changed` are typed statuses the CLI renders honestly, never an error that breaks a report.
- **Fail-closed on format drift** (D4/D5): an unknown OpenCode part type or record type under metadata-only aborts the import — privacy over availability, with the known cost that a future OpenCode version may temporarily break import until the map is updated.

## Amendment A (2026-09-15): message-record content locations (D5 correction) + composite RowID (D1 clarification)

**Finding (fixture evidence, all five fixtures).** D5's session row lists `summary_diffs`
as stripped content — correct per the dossier's session table — but the column is NULL in
every captured fixture session. The populated location is `message.data.summary.diffs`
(user messages): arrays of `{file, patch, additions, deletions, status}` entries (subset
per entry) — 14 messages / 57 entries in multi-turn-build, 28 messages / 43 entries in
subagent-tree. Under the pre-amendment map the `message` record type passed through
untouched, so those patches — full file-edit text, the exact content class this ADR
exists to exclude — entered metadata-only stores. A leak; closed here. Three observed
shapes force precision: (a) 19 messages carry `diffs` as a plain *string* (fixture
anonymization collapses oversized arrays to `[TRUNCATED-FIXTURE-ONLY]`; string is also
OpenCode's own at-rest form — the session-level `summary_diffs` column is TEXT);
(b) 6 compaction-agent messages carry `"summary": true` — a flag, not a container;
(c) an undocumented content-bearing field exists: `message.data.error` —
`{"name":"MessageAbortedError","data":{"message":"Aborted"}}`, 3 rows in subagent-tree —
the message-level sibling of the already-stripped `part.data.state.error`.

**Decision — D5 amendment.** The message row joins the content-field map:

| Record | Content fields stripped | Kept as metadata |
|---|---|---|
| `message` | `data.summary.diffs[].patch` (ref per entry); `data.summary.diffs` whole-value when not an array (text form); `data.error.data.message` | `diffs[].file/additions/deletions/status`, `error.name`, and everything else: `role`, `time`, `agent`, `mode`, `model{…}`, `modelID`, `providerID`, `variant`, `parentID`, `path`, `cost`, `tokens`, `finish` |

The split follows D5's established field-level granularity: patch parts keep `files[]`,
tool parts strip `state.input/output/error` and keep `state.title`. The patch text is
content; `file` is a path, and D5 keeps paths everywhere; `additions`/`deletions`/
`status` are the per-file twins of the session rollups D5 keeps
(`summary_additions/deletions/files`). Keeping them preserves per-turn file metrics in
metadata-only — that mode's entire point.

**Location vocabulary (fail-closed, both modes — the part-type precedent):** diff-entry
keys ⊆ {file, patch, additions, deletions, status}; `summary` object keys = {diffs};
`error` keys = {name, data}; `error.data` keys = {message}. Any unknown key aborts the
import: diff entries and the error object are typeless records — their keyset *is*
their type, and an unknown key could name a new content location that silent
pass-through would leak.

**Value-shape rules under known locations:** a non-empty text-capable value under a
*pure-content* location (`diffs`, a diffs entry, `patch`) that is not the observed
shape → whole-value ref (strip; privacy > availability). `summary`, `error`,
`error.data` are *mixed containers* (metadata and content inside): a text-capable
non-object value there is unclassifiable → fail-closed. Text-incapable scalars (null,
bool, number — e.g. `"summary": true`) pass: they provably hold no text. Empty values
(`""`, `[]`, absent key) pass. Rows with nothing stripped return original bytes
untouched and zero refs.

**D1 clarification — composite native primary keys.** `RowID` is the source row's
native primary key as the source schema defines it. Single-column PKs
(session/message/part `id`) pass through verbatim. Composite PKs encode as their
columns joined by `":"` in schema order — `todo` is (`session_id`, `position`), so
`RowID = "<session_id>:<position>"`. Consumers parse from the right (the final
`:`-suffix is the integer position; everything before is the session id) — unambiguous
even if a session id ever contained `:`. SQLite's implicit `rowid` is rejected as a
RowID source: storage-internal, not agent-native identity, unstable across
VACUUM/restore — a re-read could fetch the wrong row. The shipped todo encoding
conforms; no reissue needed.

**Consequences.**
- The session `summary_diffs` map row stands (the dossier observed the column as diff
  text) but no current fixture exercises it; its only live proof is the synthetic
  test. Honest evidence note, not a map change.
- Several refs now share one (`Table`, `RowID`) — up to N patch refs per message, up
  to 3 state refs per tool part (pre-existing). `SHA256` is the disambiguator: #8's
  resolver locates a value inside a row by hash-match over the row's content
  locations, never by position.
- The D5 hash rule applies per stripped value: raw `json.RawMessage` bytes of each
  `patch` / string-`diffs` / `error.data.message` — never re-marshaled.
- Dossier addendum required in this fix round: §3.1 gains the diffs entry shape,
  populated-on-user-messages, `summary: true`, `variant`; §3.2 gains `error`.
- The fixture script collapses oversized `diffs` arrays to a string — the fixtures
  legitimately exercise the whole-value branch (19 rows: 4 multi-turn-build,
  14 subagent-tree, 1 tool-heavy).

**Test obligations (fix round).**
- Fixture-driven metadata-only proof over every message row of every fixture:
  per-entry `patch` ref-wrapped with independently recomputed SHA256/Bytes (test
  recomputes from source raw bytes), `file/additions/deletions/status` preserved;
  string-`diffs` rows whole-value refs; `error.data.message` refs with `error.name`
  preserved; `summary:true` and `{"diffs":[]}` rows byte-untouched, zero refs.
- Pinned drift-alarm constants (ruling-time): message-record refs per fixture —
  multi-turn-build 61 (57 patch + 4 string), subagent-tree 60 (43 + 14 + 3 error),
  tool-heavy 1, single-turn 0, cache-heavy 0. A tripped constant is a deliberate
  review, never a silent pass.
- Content-local: byte-identical payloads; refs equal to metadata-only's per row.
- Fail-closed synthetics for every new vocabulary violation (unknown entry key,
  unknown `summary`/`error`/`error.data` key, `summary`/`error`/`error.data` as
  string), in both modes.
- The subtask-5 leak scan samples content bytes from *every* content location —
  per-entry patch text, string-`diffs` bytes, `error.data.message` — matched as exact
  raw JSON bytes, not naive substrings (`"Aborted"` collides with the surviving
  `MessageAbortedError`; exact-byte sampling is load-bearing).
