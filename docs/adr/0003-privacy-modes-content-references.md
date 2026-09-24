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
