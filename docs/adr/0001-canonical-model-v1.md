# ADR-1: Canonical Model v1 — Hybrid OTel Trace/Span/Event Types with Provenance

**Status:** Accepted (owner-approved 2026-09-14; DDD layout amendment included below)
**Architect:** Architect agent (`ollama-cloud/glm-5.3`) · **Issue:** #5 · **Inputs:** OpenCode dossier v1 (issue #4), Brainstorming Report §4/§6/D5/D15/D23, Final Review §5/§12, fixtures in `testdata/fixtures/opencode/`

## 1. Context

v0.1 needs the canonical in-memory model that every adapter normalizes into and every metric reads from. The decision log fixes the frame: Trace is the structural root (D23); hybrid OTel (D15); provenance on every computed value; raw events immutable with unknown fields preserved (D5); agent-neutral core; "Task" is never structural. The OpenCode dossier is the sole evidence base for v0.1 — every type below is evidenced by it; everything else is deferred explicitly.

## 2. Vocabulary decision — OTel adopted vs. AgentLens extensions

Naming rule: attributes that adopt OTel GenAI semantic conventions keep **exact OTel names** (`gen_ai.*`); coding-agent semantics OTel does not define use the `agentlens.*` namespace. This is the "not a veneer" test: where OTel has a word, we use theirs.

| Concept | Vocabulary | Rationale (one line) |
|---|---|---|
| Trace / span parent-child tree | **OTel adopted** (structural pattern) | The trace/span model is OTel's core shape; we adopt it, not invent one |
| `model_call` span | **OTel adopted**: `gen_ai.operation.name`, `gen_ai.provider.name`, `gen_ai.request.model`/`gen_ai.response.model`, `gen_ai.response.finish_reasons`, span name `{operation} {model}`, span kind CLIENT | GenAI inference client spans exist and fit exactly |
| Token usage | **OTel adopted**: `gen_ai.usage.input_tokens`, `output_tokens`, `cache_read.input_tokens`, `cache_write.input_tokens` | Exact matches in the current registry (verified) |
| Reasoning tokens | **Extension** `agentlens.usage.reasoning_tokens` | No reasoning-usage attribute exists in GenAI semconv (verified); migrate via ADR if upstream adds one |
| Total tokens | **Not stored** — derived sum | No OTel attribute; derivable; avoids dual truth |
| `tool_call` span | **OTel adopted**: `gen_ai.tool.name`, `gen_ai.tool.call.id`, execute_tool semantics, span kind INTERNAL, span name `execute_tool {tool}` | OTel's execute_tool span is precisely this |
| Tool input/output | **Extension**: `agentlens.content.sha256`, `agentlens.content.bytes`, ref to raw event | OTel stores content opt-in; D28 is stricter — references only |
| `session` span | **Extension** kind `agentlens.span.session` + OTel `gen_ai.conversation.id` attribute | OTel has no session span kind; conversation.id is the closest adopted word |
| `turn` span | **Extension** kind `agentlens.span.turn` | No OTel analog; boundaries observed via `parentID` |
| Agent identity | **OTel adopted**: `gen_ai.agent.name` (per message/call, observed) | OTel has agent attributes; no separate `agent_execution` span kind needed in v0.1 |
| Subagent execution | Child **session** spans, parent edge observed (`parent_id`) | The dossier evidences sessions, not a separate agent-run entity |
| Compaction | **Extension** event kind `session_compaction` + **OTel adopted** `gen_ai.conversation.compacted=true` on the first subsequent model_call | OTel's attribute exists and is adopted; the event itself is ours |
| `file_edit` | **Extension** event kind (files + patch hash, observed) | Patch parts are point facts without duration — events, not spans |
| Cost | **Extension** `agentlens.cost.*` | No OTel cost semconv (verified); OpenCode reports cost as 0 → **unavailable**, estimated later via LiteLLM (#11) |
| Capability flags | **Extension** type (per source format) | No OTel analog |
| Provenance | **Extension** — the core differentiator | No OTel analog; OTel has no provenance semantics |
| `file_read` | Not a distinct kind — `tool_call` with `gen_ai.tool.name=read` | Read tools are evidenced as tool calls; taxonomy is adapter attributes |
| `test_run` | **Deferred** (v0.2 smells; derived from bash output per dossier §11) | Not observed as a distinct entity |
| `human_intervention` | **Deferred** | Not evidenced by the OpenCode dossier; prime candidate for the Claude Code dossier |
| `skill_invocation`, `git_change`, `workflow_outcome` | **Deferred** | Unevidenced in dossier (summary diffs are content refs, not git ops) |
| `task_inference`, `session_completeness` | **Deferred** (v0.2 / metrics issue) | Derived views; D23 forbids task types in the model |

**OTel span-kind mapping** (for future OTLP export): `model_call`→CLIENT, `tool_call`→INTERNAL, `session`/`turn`→INTERNAL (neutral default; OTel GenAI uses operation names, not span kinds, for semantics — we mirror that).

## 3. Answers to the dossier's §12 questions

### Q1 — Step-finish as model call, message tokens as rollup: **Confirmed, with sharpened semantics**

Fixture verification: step-finish↔assistant-message is **1:1** (365/365 messages with step-finish have exactly one; zero multi-step), and message tokens == step-finish tokens exactly (80/80 sampled, `multi-turn-build`). So:

- **One `model_call` span per assistant message** (= per step in all observed data). Identity = native assistant message id. Start/end = `time.created`/`time.completed` (observed; ~1% lack `completed` → EndTime zero = incomplete, never fabricated).
- **`step-start`/`step-finish` parts are not spans** — they are attributes/evidence on the enclosing model_call (workspace snapshot hash → `agentlens.workspace.snapshot`; step reason corroborates `gen_ai.response.finish_reasons`).
- **Message-level tokens are per-call usage, not a separate rollup** — in observed OpenCode data they equal the step's usage. Turn/session/trace rollups are **AgentLens-derived sums**; OpenCode's stored session rollups are kept as raw source data for cross-validation, never as canonical values.
- Adapter-contract rule: if a future format emits multiple steps per message, the adapter issues one model_call span per step — the canonical model stays stable, adapters absorb drift.

### Q2 — Keep `variant` as a model-identity attribute: **Yes**

`variant` (`"max"`) is observed in session-level agent blobs and distinguishes serving configs of the same `(providerID, modelID)`. It rides as `ModelIdentity.Variant Value[string]` — session-level observed, per-call **unavailable** (assistant records don't carry it; we never fabricate per-call variant). Grouping key stays `(ProviderID, ModelID)`; variant is descriptive metadata, not part of the key — pricing (#11) may join on it.

### Q3 — Subagent child sessions as observed task units: **Confirmed**

One child session = one observed task unit: boundary observed (`parent_id`), task prompt = first user message (content referenced per D28, re-read on demand), agent identity and models observed per child. The v0.2 task-inference view treats child sessions as directly observed units and only infers segmentation *within* sessions. The v0.1 model needs no task types — it merely preserves what inference will need (child session spans, observed parent edges, agent attributes, prompt references). All present.

## 4. Canonical type inventory

Single package **`internal/model`** (stdlib only — `encoding/json`, `time`, `errors`; zero new dependencies for this issue). `internal/` not `pkg/` because the SDK promise is v1.0; promotion is a rename, not a redesign. Adapters populate structs directly and call `Validate()` — **no builder abstraction**.

### Provenance & values

```go
type Provenance uint8

const (
	ProvenanceUnavailable Provenance = iota // zero value: the honest default
	ProvenanceObserved
	ProvenanceDerived
	ProvenanceEstimated
	ProvenanceInferred
)

func (p Provenance) String() string

type Value[T any] struct {
	V          T
	Provenance Provenance
	SourceRef  string
}

func Observed[T any](v T, ref string) Value[T]   // rejects empty ref
func Derived[T any](v T) Value[T]                // intra-trace derivations
func Estimated[T any](v T, ref string) Value[T]  // ref = pricing snapshot version
func Inferred[T any](v T, ref string) Value[T]   // ref = inference run id
func Unavailable[T any]() Value[T]

func (v Value[T]) Validate() error
```

Rules enforced at construction + `Validate`: `observed`/`estimated`/`inferred` **require** non-empty `SourceRef`; `unavailable` carries a meaningless V and empty ref; zero-value `Value` is honestly `unavailable`. JSON shape: `{"value":…, "provenance":"observed", "source_ref":"…"}` — the `provenance` key is always emitted.

### Kinds & status

```go
type SpanKind uint8

const (
	KindSession SpanKind = iota + 1
	KindTurn
	KindModelCall
	KindToolCall
)

func (k SpanKind) String() string
func (k SpanKind) OTelKind() string

type EventKind uint8

const (
	EventFileEdit EventKind = iota + 1
	EventSessionCompaction
	EventTaskMarker
)

func (k EventKind) String() string

type Status uint8

const (
	StatusUnset Status = iota
	StatusOk
	StatusError
)
```

Exactly four span kinds and three event kinds — each evidenced by the dossier. **No `agent_execution` kind**: agent identity is `gen_ai.agent.name` on spans; subagent runs are child session spans. Unknown source vocabularies (e.g., `finish` values) stay **strings**, never enums — the dossier's quirk #10 says the vocabulary grows.

### Usage, model identity, cost

```go
type Usage struct {
	InputTokens      Value[int64] // gen_ai.usage.input_tokens
	OutputTokens     Value[int64] // gen_ai.usage.output_tokens
	ReasoningTokens  Value[int64] // agentlens.usage.reasoning_tokens (extension)
	CacheReadTokens  Value[int64] // gen_ai.usage.cache_read.input_tokens
	CacheWriteTokens Value[int64] // gen_ai.usage.cache_write.input_tokens
}

type ModelIdentity struct {
	ProviderID string // gen_ai.provider.name
	ModelID    string // gen_ai.response.model
	Variant    Value[string] // agentlens.model.variant (extension); unavailable per-call
}

type Cost struct {
	Amount   Value[float64] // agentlens.cost.amount (extension)
	Currency string
}
```

Per-field `Value` is deliberate: providers report token fields partially (absent ≠ zero). **Adapter invariant**: JSON-absent field → `Unavailable` (V=0); JSON-present zero → `Observed` (V=0). OpenCode's all-zero cost fields → `Unavailable` per dossier §9.1, never "observed 0".

### Core structures

```go
type Span struct {
	ID            string
	TraceID       string
	ParentID      string // "" only for the root session span
	Kind          SpanKind
	Name          string // OTel convention: "chat glm-5.3", "execute_tool bash", "turn 2"
	StartTime    time.Time
	EndTime      time.Time // zero = incomplete; never fabricated
	Status        Status
	StatusMessage string
	Attributes    map[string]any // OTel names where adopted, agentlens.* extensions
	Model         *ModelIdentity // model_call only
	Usage         *Usage          // model_call only
	Cost          *Cost           // session + model_call only
	FinishReason  Value[string]  // model_call only; string, open vocabulary
	SourceRef     string          // raw event id(s) this span was derived from
}

type Event struct {
	ID         string
	SpanID     string // must resolve to a span in the trace; no trace-level events
	Kind       EventKind
	Time       time.Time
	Attributes map[string]any
	SourceRef  string
}

type Trace struct {
	ID           string
	Agent        string // "opencode" — namespace for ids and source refs
	StartTime    time.Time
	EndTime      time.Time
	Attributes   map[string]any
	Source       SourceMetadata
	Capabilities Capabilities
	Spans        []*Span
	Events       []*Event
}

type SourceMetadata struct {
	RootSessionID    string // native root session id, observed
	AgentVersion     string // "1.18.30", observed
	ProjectDirectory string
	ImportedAt       time.Time
	AgentLensVersion string
}

type Capabilities struct {
	PerCallUsage          bool
	CacheTokenUsage       bool
	ReasoningTokenUsage   bool
	ToolCallDuration      bool
	FileEditEvents        bool
	CompactionEvents      bool
	TaskMarkers           bool
	SubagentSessions      bool
	SessionUsageRollups   bool
	AgentIdentityPerMessage bool
	ProviderCost          bool
	ModelVariant          bool
}

func (t *Trace) Validate() error // full-tree validation; errors.Join for all violations
```

**Kind-conditional field rules** (validated): `Model`/`Usage`/`FinishReason` non-nil iff `KindModelCall`; `Cost` on `KindSession`/`KindModelCall` only; parents — `session` ← session-or-nothing (single root per trace), `turn` ← session, `model_call` ← turn, `tool_call` ← model_call; `file_edit`/`session_compaction` events attach to the span of the message containing the part (observed `message_id`); `task_marker` attaches to its session span.

**No rollups on spans** — Usage exists only on model_call; turn/session/trace sums are derived views computed by the metrics layer. One truth, rebuildable from raw.

### Raw events (where agent-native payloads live)

```go
type RawEvent struct {
	ID         string
	TraceID    string
	Agent      string
	RecordType string // e.g. "session", "message", "part", "todo"
	Payload    json.RawMessage // verbatim bytes; unknown fields preserved by construction
	CapturedAt time.Time
}
```

`RawEvent` is the immutable-truth wrapper: `Payload` stores agent-native JSON **verbatim** (D5), validated with `json.Valid` only — never interpreted, never executed (untrusted input). Spans point back via `SourceRef` = raw event id. Storage lives in issue #6; the type and contract land now because every `SourceRef` depends on it.

### IDs — adapter-issued, deterministic, opaque

- **`Trace.ID`**: one trace = one root agent-native session **plus all descendant sessions** (subagent tree — verified: 26 children under one root). AgentLens-issued ids are required because a trace spans multiple native sessions and native id spaces could collide across agents. Deterministic derivation from `(agent, root native session id)` makes re-import idempotent (upsert-no-op).
- **`Span.ID`/`Event.ID`/`RawEvent.ID`**: opaque strings, **issued by the adapter** as deterministic functions of native record identity (turns derive from their user-message id). Deterministic ⇒ stable across re-parses ⇒ derived views and references survive rebuilds. Core never interprets id formats (adapter isolation); `Validate` enforces uniqueness within the trace and edge integrity.
- **Dangling native parents** (e.g., tool-heavy fixture's session whose parent is not imported): the adapter creates **no span edge** and keeps the observed reference as an attribute (`agentlens.session.parent_ref`, observed). `Validate` requires every `ParentID` to resolve — the adapter absorbs the dangle.

### Timestamps & time provenance

Epoch **ms in** (OpenCode), `time.Time` **UTC canonical**, RFC 3339 with nanoseconds on the wire (ms fidelity preserved; tested). EndTime zero = incomplete/open — allowed everywhere, never fabricated.

**Time provenance is a function of kind**, documented here rather than wrapped per-field (provenance by taxonomy, not redundancy):

| Kind | StartTime | EndTime |
|---|---|---|
| session | observed (`time_created`) | observed (`time_updated`, row-update semantics) |
| turn | observed (user message time) | **derived** (max of member model_call ends) |
| model_call | observed (`time.created`) | observed (`time.completed`) or zero (incomplete) |
| tool_call | observed (`state.time.start`) | observed (`state.time.end`) |
| trace | derived (min session start) | derived (max session end) |

`Value[T]` wrapping is reserved for values whose provenance **varies at runtime** (usage fields, cost, variant) — exactly where fabrication risk lives.

### Serialization

JSON tags on all types, snake_case, enums as their lowercase canonical strings. Purpose: the storage layer (#6) and `agentlens share` consume the same wire shape; round-trip stability is a tested invariant. No `omitempty` on `provenance` — a missing provenance is a schema violation, not a state.

## 5. Dossier fit check (issue Done-when #3)

| OpenCode record | Canonical target | Provenance |
|---|---|---|
| `session` row | `KindSession` span + `SourceMetadata` | observed |
| `session.parent_id` | span parent edge (or `agentlens.session.parent_ref` when dangling) | observed |
| user message | `KindTurn` span start | observed |
| assistant message | `KindModelCall` span (model, usage, times, finish, `gen_ai.agent.name`) | observed |
| `step-start`/`step-finish` | attributes on the model_call (`agentlens.workspace.snapshot`, reason corroboration); usage source | observed |
| tool part | `KindToolCall` span (name, call id, duration, status; input/output as sha256+bytes+ref only) | observed |
| patch part | `EventFileEdit` (files, patch hash) | observed |
| compaction part | `EventSessionCompaction` + `gen_ai.conversation.compacted` on next model_call | observed |
| todo rows | `EventTaskMarker` on session span (status/priority/position; content = ref only, re-read on demand) | observed (final state only — update history unavailable, documented) |
| session token rollups | raw source data only; cross-check, never canonical | observed-as-stored |
| `cost` fields | `Cost.Amount` = `Unavailable` (all-zero observed → treated as unreported) | unavailable → estimated in #11 |
| text/reasoning parts | raw events only; content re-read on demand (D28) | observed existence, content referenced |
| session `agent` blob `variant` | session-level `ModelIdentity.Variant` | observed (per-call: unavailable) |
| `event` table | not consumed in v0.1 (dossier §8) | — |
| `auth.json` | **never read** (adapter hard rule) | — |

**No forced fits.** Gaps recorded as explicit unavailable: provider cost, per-call variant, per-part reasoning token counts, todo update history, todo→turn attribution (derived at view time).

## 6. Explicitly NOT in v0.1 canonical model

- Task types / task inference (v0.2 derived view; D23 — never structural)
- `test_run` (v0.2, derived from tool output), `human_intervention` (unevidenced; revisit with Claude Code dossier), `file_read` as a kind (tool taxonomy), `skill_invocation`, `git_change`, `workflow_outcome`
- Completeness score computation (metrics issue; this model supplies the inputs: incomplete EndTimes, unavailable usage fields)
- Estimated cost computation (#11; the `Cost` type exists, adapter marks it unavailable)
- Smells, recommendations, labels (R5 reserves storage only)
- OTLP export (the `OTelKind()` mapping keeps the door open; no collector)

## 7. Consequences

- One package, ~10 small files, zero dependencies — the schema is the spine and it's thin on purpose.
- Adapters are held to a hard contract: deterministic ids, kind-conditional fields, absent≠zero token discipline, references-not-content, `Validate()` before persist.
- Rollback path: this is the v0.1 model; Claude Code (v0.2) is the schema-validation milestone — any type it can't produce honestly triggers an ADR revision, not a hack.
- Upstream drift risk accepted (GenAI semconv is `Development` status): adopted names are confined to `gen_ai.*` attributes whose semantics are stable; migration is an ADR with alias support.

---

---

## 8. Code organization amendment (owner-approved 2026-09-14)

Per the standing DDD architecture (AGENTS.md §3), the canonical model lives in **`internal/domain`** — the domain layer at the center: imported by everything, importing nothing outside stdlib. The mini-plan's file names map to `internal/domain/*.go`. Adapters under `internal/ingest/<agent>/` populate domain types through the adapter contract (anti-corruption layer); `internal/normalize`, `internal/analysis`, `internal/storage` (repositories behind domain-defined interfaces), and `internal/cli` build outward from this core. Tactical-DDD ceremony (domain events, CQRS) is explicitly deferred until evidence demands it.
