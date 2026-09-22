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

The `schema_version` table is bootstrapped by the migration runner itself (`CREATE TABLE IF NOT EXISTS`, before the version read), so it is not part of `0001_init.sql`:

```sql
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
	agent          TEXT NOT NULL,
	record_type    TEXT NOT NULL,
	captured_at    INTEGER NOT NULL,
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

## Amendment A (2026-09-15): raw-event envelope

Found during #6 implementation, before any read code shipped. D6's `raw_events`
DDL persisted only `trace_id, id, seq, payload, payload_sha256` — but the domain
`RawEvent` (ADR-1) has six fields: `ID, TraceID, Agent, RecordType, Payload,
CapturedAt`. Under that DDL `RawEventsByTrace` could not reconstruct `Agent`,
`RecordType`, or `CapturedAt`: none are columns, and none are reliably present
in the payload — the payload is verbatim agent-native bytes that storage never
interprets (adapter isolation). A lossy evidence layer breaks the principle D7
and AGENTS.md rest on — *everything derived is rebuildable from raw events*:
`RecordType` is first-class parse-stage metadata that `internal/normalize` (#9)
and reprocess (v0.2) dispatch on; recovering it would mean re-running adapter
parsing inside storage. The Implementer correctly halted rather than improvise.

**Decision.** `raw_events` persists the full raw-event envelope as first-class
columns: `agent TEXT NOT NULL`, `record_type TEXT NOT NULL`, `captured_at
INTEGER NOT NULL` (epoch ms via `domain.ToEpochMillis`/`FromEpochMillis` — the
schema-wide time contract). `payload` stays a verbatim `BLOB`; the envelope is
written alongside the bytes, never re-marshaled into them. This supersedes D2's
"columns only where SQL needs keys" **for the raw layer only**: D2 was written
for the derived, rebuildable tables; raw events are the base layer of truth and
must be self-describing without interpretation. `agent` is required precisely
because D7 deliberately gives `raw_events` no FK to `traces` — a raw event must
identify its agent even when its trace row is absent (normalization failed, #9).

**Delivery: folded into `0001_init.sql`, not a new `0002` migration.** The
never-edit-after-merge freeze attaches at *merge*; `0001_init.sql` exists only
on the unmerged issue-#6 branch, no binary has shipped, and zero databases exist
in the wild — there is nothing to upgrade and no backfill to run. A `0002`
would exist solely to patch a baseline no one ever held and would permanently
misdescribe v1. The file is edited in place by a normal commit (no history
rewrite; pre-release dev databases are disposable — delete and re-open). Had
this branch been merged or any store shipped, the identical delta lands as
`0002_raw_event_envelope.sql` instead — no exceptions.

**Contract consequences.**

- `Ingest` writes the envelope columns with every insert. A zero `CapturedAt`
  is rejected at the ingest gate — capture time is always known at capture;
  never fabricate, never persist an undefined epoch conversion (fail-loud,
  same posture as the sha-conflict gate, D9).
- `payload_sha256` remains **payload-only** (SHA-256 over payload bytes). It is
  the *content* identity of the evidence; the envelope is capture *context* —
  `CapturedAt` legitimately differs between re-imports of the same record, so
  hashing it would turn every idempotent re-ingest into a false `ErrConflict`.
  Same id + same sha → full row skip, no envelope refresh (D8 immutability
  forbids the update; first-capture time is a fact, not a stale value).
- `RawEventsByTrace` returns the full envelope, reassembled `ORDER BY seq`
  (D4). Round-trip byte-identity tests now cover the envelope too: `Agent`/
  `RecordType` exact, payload `bytes.Equal` (byte-identity, not
  JSON-equivalence), `CapturedAt` asserted at ms fidelity (ms truncation is
  the schema-wide contract, same as `traces.start_time`).

**Ripple effects: none.** No index changes — every v0.1 raw-event query is by
`trace_id`, already covered by the PK and `UNIQUE(trace_id, seq)`. No trigger
changes — `BEFORE UPDATE ON raw_events` aborts any row update regardless of
which columns are touched, so immutability extends to the envelope for free.
D1 (repository contract) is untouched; this correction is what lets storage
honor it. The domain `RawEvent` and its `Validate()` are untouched (ADR-1); an
empty `Agent` still satisfies `NOT NULL` and remains an adapter-contract
matter, not a storage one.

Appendix A's `raw_events` block becomes:

```sql
CREATE TABLE raw_events (
	trace_id       TEXT NOT NULL,
	id             TEXT NOT NULL,
	seq            INTEGER NOT NULL,
	agent          TEXT NOT NULL,
	record_type    TEXT NOT NULL,
	captured_at    INTEGER NOT NULL,
	payload        BLOB NOT NULL,
	payload_sha256 TEXT NOT NULL,
	PRIMARY KEY (trace_id, id),
	UNIQUE (trace_id, seq)
);
```

## Amendment B (2026-09-15): nil-vs-empty slice semantics — Events gated, Attributes collapsed by the wire

Found during the #6 review, after the Test Engineer's adversarial pass. D3
promises byte-identity of the reassembled full-trace marshal, but SQL row
counts cannot distinguish `Events == nil` from `Events == []*Event{}`: both
persist zero event rows, and reassembly (post-fix, correctly) returns nil
for zero rows. A trace ingested with `Events: []` — legal under
`Validate()`, and the natural result of `make([]*Event, 0, n)` in an
adapter that finds no events — reassembles as nil, marshals
`"events":null` against the input's `"events":[]`, and violates D3 on a
legal input. Spans cannot hit this: `Validate` requires exactly one root
span, so a valid trace never carries an empty `Spans`. The question
generalizes to `Attributes` on trace/span/event; both directions are
ruled here.

**Decision.** The canonical model speaks one form for "no events": absent.
`Ingest` rejects an empty-non-nil `trace.Events` at the gate (fail-loud,
same posture as the Amendment A zero-`captured_at` gate). Nil and
non-empty are the only accepted shapes, and this is a contract on every
producer feeding a `domain.TraceIngestor`, not a storage implementation
detail. An empty-non-nil Events is not a fact about the observed session —
it is a construction artifact of the producing code — and Amendment A's
principle applies verbatim: never persist an undefined distinction. The
alternative (a header emptiness marker preserving both forms) was
rejected: it canonizes a producer quirk into the durable format, adds a
marker↔tree consistency invariant, and taxes every future serialization
surface (#14 share, v0.5 HTTP) with a bookkeeping bit that carries zero
observational semantics.

`Attributes` needs no gate: ADR-1's wire shape (`omitempty` on trace,
span, and event) already collapses nil and empty to the same bytes — both
omit the key, both reassemble as nil, and byte-identity holds in both
directions by construction. The distinction does not survive the wire;
nothing may depend on it.

**Contract consequences (adapters #8/#9 read this).**

- Builders may pre-allocate freely (`make([]*Event, 0, n)` + append); the
  gate judges the value at the boundary, not the construction style. A
  trace reaching `Ingest` with zero events must carry them as nil.
- The gate error is a plain error (not `ErrConflict`, not `ErrNotFound`)
  — callers cannot mistake it for idempotency machinery.
- Any implementation of `domain.TraceIngestor` enforces the same
  precondition; storage's gate is the reference behavior.
- No gate on `Attributes` (trace/span/event) and none on `Spans`
  (unreachable: `Validate` demands exactly one root span).

**Ripple effects: none.** No DDL, no domain change, no header mirror
change, no migration (zero databases exist — same delivery situation as
Amendment A). `content_sha256` is unaffected: zero events hash
identically in both shapes, and the empty shape never reaches the hash.
No byte-identity promise exists on the list returns of `ListTraces` /
`RawEventsByTrace` or on the `raw` input slice; their emptiness shape is
presentation, not contract.

**Test obligations (both directions, regression suite).**

- Events nil → reassembles nil; marshal byte-identical (`"events":null`).
- Events `[]` → `Ingest` fails with the gate error and persists nothing
  (the gate fires before the transaction begins).
- Pre-allocated-then-filled events pass (pins that capacity is not the
  crime).
- Attributes nil and Attributes `{}` (on the trace, a span, and an
  event) each round-trip byte-identically (pins the wire collapse as
  tested fact, not claim).
