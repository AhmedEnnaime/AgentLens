# ADR-2: SQLite Storage Schema, Repository Contract, and Migration Policy

**Status:** Accepted (owner-approved 2026-09-15 with the mini-plan for issue #6)
**Architect:** Architect agent (`ollama-cloud/glm-5.3`) · **Issue:** #6 · **Inputs:** ADR-1 (canonical model + serialization invariant), mini-plan for #6, OpenCode dossier §1 (store robustness posture)

## 1. Context

v0.1 needs a persistence layer for the canonical model defined in ADR-1. This ADR covers the persistence schema, the domain-declared repository contract, the migration policy, and the store lifecycle. The canonical model itself does not change — ADR-1 stands untouched; the domain change is purely additive (`repository.go`). Four forces make this an ADR rather than an issue comment:

1. `~/.agentlens/agentlens.db` outlives every binary a user ever installs; its rationale must be reconstructible in v0.3.
2. Deciding that spans/events persist as wire-JSON blobs promotes ADR-1's wire shape to the *persistence format* — a durable contract (reprocess v0.2, labels v0.3, share #14, HTTP API v0.5) that will be questioned.
3. The migration policy starts here and is frozen at v1.0.
4. R5 (labels reserved) is a decision-log entry the schema must honor.

## 2. Decision

### D1 — Repository contract: two small interfaces in `internal/domain`

Ingest and read consumers are disjoint (write pipeline vs CLI); segregation has real consumers, and one god-interface does not. Composing both in `*storage.Store` is storage-internal. Exactly four operations — `Ingest`, `TraceByID`, `ListTraces`, `RawEventsByTrace` — are what v0.1–v0.2 consume. No standalone `AppendRawEvents` (no consumer until a partial-parse flow exists), no update/delete methods, no paging. Sentinel errors `ErrNotFound`/`ErrConflict` live in domain; storage maps SQLite errors at the boundary (ACL discipline).

### D2 — Persistence model: per-entity rows, wire-JSON blob per entity, minimal key columns

Spans and events are derived and rebuildable (ADR-1 D5) — normalizing them into 15 SQL columns buys nothing in v0.1 because no SQL-level analytics exist (`internal/analysis` is pure Go over domain objects). The blob *is* the ADR-1 wire shape, so the serialization invariant (byte-stable round-trip) becomes the persistence invariant with zero translation. Columns exist only where SQL needs keys: identity, trace membership, seq, and the four list-filter/sort fields on `traces`. Query columns are added by additive migration only when a real query demands them.

### D3 — No dual truth: the trace header blob excludes spans/events

`Trace` marshals with `spans`/`events` inline, so storing a full-tree trace blob *plus* span rows would duplicate the tree. Instead `traces.header` holds wire-JSON of `Trace` minus `Spans`/`Events` (storage-internal mirror struct, identical field order/tags), spans and events live only in their rows, and `TraceByID` reassembles. Byte-identity of the reassembled full-trace marshal is a tested invariant — ADR-1 §Serialization extended through SQLite.

### D4 — Slice order is a persisted contract: `seq` on spans, events, raw events

`UNIQUE(trace_id, seq)`; reads reassemble `ORDER BY seq`; raw-event seq continues from `MAX(seq)+1` per trace so a re-imported grown session appends rather than reorders. Without this, byte-identical round-trips are luck.

### D5 — Provenance persists as its wire form inside the blobs

Every `Value[T]` marshals as `{value, provenance, source_ref}` (provenance key always present). No provenance columns — v0.1 has zero queries over provenance, and copying it into columns would create driftable dual truth.

### D6 — Schema (migration `0001_init.sql`)

Full DDL in Appendix A. Indices justified by v0.1 query patterns only: list-by-project/date (`idx_traces_project_start`), fetch-by-id (PKs), spans/events/raws-by-trace (the `UNIQUE(trace_id, seq)` indexes). Counts for `TraceSummary` come from `GROUP BY` at query time — no denormalized count columns to drift.

### D7 — Composite PKs; raw_events points forward, not upward

ADR-1 guarantees id uniqueness only *within* a trace — global PKs would invent a contract adapters never signed. `raw_events` deliberately has no FK to `traces`: raw events are the base layer of truth (parse stage, #8) and may legitimately exist before or without their derived trace (normalization failed, #9); the dependency direction is trace→raw evidence, never raw→trace. Integrity of that forward reference is checked in Go at read/reprocess time, not by SQL.

### D8 — Immutability enforced at three layers, defense in depth

(1) API: no mutator methods on raw events — `Ingest` only appends (`INSERT … ON CONFLICT DO NOTHING`); (2) DB: two `RAISE(ABORT)` triggers make even ad-hoc `UPDATE`/`DELETE` fail; (3) this ADR (documentation). The trigger is what makes the guarantee *testable*, not aspirational. Derived tables (spans/events/traces) are legitimately replaceable — reprocess's v0.2 mechanism lands for free.

### D9 — Idempotency = deterministic ids + content hash + conflict detection

`Ingest` runs `trace.Validate()` + each `raw.Validate()` + `raw.TraceID == trace.ID` gate first (untrusted-input discipline), then in one transaction:

- (a) raw events — existing id with same `payload_sha256` → skip; same id, different sha → `ErrConflict`, rollback (an adapter-contract violation, never silently absorbed); new → append with continued seq;
- (b) derived tree — `content_sha256` = SHA-256 over span+event blob bytes in seq order; unchanged → tree untouched; changed/absent → delete events-then-spans for the trace, reinsert (reprocess semantics).

Re-ingest of the same session is a no-op on all rows except `traces.header`, which refreshes `source.imported_at` (last-import-wins — the only write).

### D10 — Migrations: embedded numbered SQL, forward-only, refuse-newer

`internal/storage/migrations/0001_init.sql…` via `go:embed` (single-binary distribution; numbered files, never edited after merge). Runner: read `MAX(version)`, apply each higher migration in its own transaction, stamp `schema_version`. Gaps in sequence → corrupt-store error. DB version > binary's highest → refuse to open ("database created by a newer AgentLens") — forward-only means old binaries never touch newer DBs.

### D11 — Lifecycle: `~/.agentlens/agentlens.db`, env override, WAL

`storage.Open(path)` creates the dir (`0700` — the DB references real paths), opens, applies pragmas — `journal_mode=WAL`, `busy_timeout=5000`, `foreign_keys=ON`, `synchronous=NORMAL` — then migrates. `AGENTLENS_DB` overrides the path (tests use `t.TempDir()`; no test ever touches `~`). WAL matches the robustness posture of the OpenCode store we ourselves read (dossier §1). Concurrency: WAL readers never block; two concurrent ingest writers serialize via busy_timeout (single-user CLI reality — accepted for v0.1).

## 3. Consequences

- One new third-party dependency: `modernc.org/sqlite` (pure Go, zero CGO), driven through `database/sql` with DSN `file:<path>?_pragma=...`.
- The raw-event store is append-only at the DB layer; right-to-erasure (a future `purge`) is its own issue, not an in-place mutation.
- Spans/events/traces are replaceable via tree-replace (D9), which is reprocess v0.2's mechanism landing now.
- The migration discipline is frozen: numbered SQL files, never edited after merge, forward-only.

## Appendix A — DDL (`0001_init.sql`)

```sql
CREATE TABLE schema_version (
	version    INTEGER PRIMARY KEY,
	applied_at INTEGER NOT NULL
);

CREATE TABLE traces (
	id                TEXT PRIMARY KEY,
	agent             TEXT NOT NULL,
	root_session_id   TEXT NOT NULL,
	project_directory TEXT NOT NULL,
	start_time        INTEGER NOT NULL,
	end_time          INTEGER,
	content_sha256    TEXT NOT NULL,
	header            TEXT NOT NULL
);
CREATE INDEX idx_traces_project_start ON traces (project_directory, start_time DESC, id);

CREATE TABLE spans (
	trace_id TEXT NOT NULL REFERENCES traces (id),
	id       TEXT NOT NULL,
	seq      INTEGER NOT NULL,
	blob     TEXT NOT NULL,
	PRIMARY KEY (trace_id, id),
	UNIQUE (trace_id, seq)
);

CREATE TABLE events (
	trace_id TEXT NOT NULL REFERENCES traces (id),
	id       TEXT NOT NULL,
	span_id  TEXT NOT NULL,
	seq      INTEGER NOT NULL,
	blob     TEXT NOT NULL,
	PRIMARY KEY (trace_id, id),
	UNIQUE (trace_id, seq),
	FOREIGN KEY (trace_id, span_id) REFERENCES spans (trace_id, id)
);

CREATE TABLE raw_events (
	trace_id       TEXT NOT NULL,
	id             TEXT NOT NULL,
	seq            INTEGER NOT NULL,
	payload        BLOB NOT NULL,
	payload_sha256 TEXT NOT NULL,
	PRIMARY KEY (trace_id, id),
	UNIQUE (trace_id, seq)
);

CREATE TABLE labels (
	trace_id   TEXT NOT NULL REFERENCES traces (id),
	key        TEXT NOT NULL,
	value      TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (trace_id, key)
);

CREATE TRIGGER raw_events_immutable_update BEFORE UPDATE ON raw_events
BEGIN SELECT RAISE(ABORT, 'raw_events is immutable'); END;
CREATE TRIGGER raw_events_immutable_delete BEFORE DELETE ON raw_events
BEGIN SELECT RAISE(ABORT, 'raw_events is immutable'); END;
```
