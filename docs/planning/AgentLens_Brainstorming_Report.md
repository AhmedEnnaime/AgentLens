# AgentLens — Architecture Brainstorming Report & Working Decisions

**Status:** Living document — brainstorming / discovery phase. Nothing here is final.
**Date:** 2026-09-13
**Inputs:** `AI_Coding_Session_Observatory.md` (original vision — treated as input, **not** as authority) · `AgentLens_FINAL_ARCHITECTURE_REVIEW.md` (owner decisions, 2026-09-14 — resolves Q1–Q15; consolidated into Part VII)
**Purpose:** Consolidated architecture record — the original vision review, all decisions taken (including the owner's final decisions from `AgentLens_FINAL_ARCHITECTURE_REVIEW.md`), and the small set of remaining open items.

---

# Part I — Review of the Original Vision

## 1. Understanding of the idea

AgentLens is intended to be a **standalone, local-first, agent-agnostic tool** that:

1. **Captures** coding-agent sessions (OpenCode first, then Claude Code, Codex, others) via adapters — post-hoc from session files, later live.
2. **Normalizes** them into a canonical event model with strict provenance (`observed / inferred / estimated / unavailable`).
3. **Computes deterministic metrics** — tokens, cache, cost, latency, workflow structure, agent/model attribution.
4. **Adds an LLM analyst layer** (opt-in) to interpret, explain, and eventually recommend.
5. **Matures into** an optimization / model-evaluation / routing platform grounded in the user's own historical evidence, with human approval at every stage.

Guiding principle from the original doc, kept and endorsed:

> **The observer measures reality; the LLM explains reality.**

And the four-question ladder that should organize the roadmap:

```text
What happened?  →  Why did it happen?  →  How can it improve?  →  What should it do next time?
```

## 2. Strongest parts of the current vision (keep these)

- **Provenance discipline** — field-level `observed / inferred / estimated / unavailable`. Ahead of most commercial observability tools. Never cut this.
- **"Optimize outcomes, not token counts"** — correct and anti-obvious; most tools quietly optimize tokens because tokens are easy to measure.
- **Local-first privacy posture** — this is the strategic differentiator, not a nicety. It is what separates AgentLens from hosted LLM-observability platforms.
- **Staged routing** — refusing to auto-route in v1 is the right restraint.
- **The anti-patterns section** — "more agents = better", "same-model self-review" show real thinking about agent-ecosystem failure modes.
- **The four-question ladder** — should literally gate the roadmap layers (see Part IV).

## 3. Challenges to the assumptions

### 3.1 The document never names a user

Three genuinely different products hide behind "developers":

| Persona | Wants | Product shape |
|---|---|---|
| **Individual agent user** | "Why is my bill $200/mo? What did that session actually do? Which model should I use?" | CLI + local reports + inspection |
| **Team lead** | Org spend, who uses what, policy | Server, dashboards, auth, multi-tenancy |
| **AI engineer / researcher** | Model comparison, experiments, evals | Experiment framework, statistics, notebooks |

The original doc designs for persona 1 but roadmap-plans for personas 2 and 3. **Persona 1 is the only viable first target** — you can be your own user, distribution is free, sales cycle is zero. Consequence: no server, no auth, no multi-tenancy, no experiment UI for a long time.

### 3.2 The hardest problem is hand-waved: outcome measurement

"Same or better engineering outcome" rests on attributing outcomes to sessions, which is genuinely hard:

- Humans often fix code *after* the session ends — model failure or normal collaboration?
- CI runs later, on a branch, mixed with human commits.
- Tasks are never one-to-one comparable; "similar tasks" requires the inference layer that doesn't exist yet.

Honest ranking of outcome **fidelity tiers**:

1. **In-session signals** (fully available now): tests run during session + results, lint/typecheck, retries, human interrupts/edits mid-session, final diff produced.
2. **Session-adjacent**: diff stats, whether the diff survived to a commit/PR.
3. **Post-hoc**: CI status of resulting commits, PR review findings, reverts.

Tier 1 is buildable now. Tier 3 requires git/CI integration with attribution ambiguity. And the cheapest high-fidelity signal: **explicit human labeling** — `agentlens label <session> good|bad` takes two seconds and is the bootstrap ground truth the learning loop needs. Add it early.

### 3.3 Routing has a cold-start problem the roadmap hides

"Documentation tasks historically did fine on Flash" requires task classification, months of history, outcome ground truth, and cross-session comparability. For a personal tool, **n will be 3–30 for a long time, not 300**.

Consequence to internalize: for the first 6–12 months, recommendations will be **deterministic smoke detection** (repeated context %, retry loops, cache ratio, compaction frequency), *not* model routing. The LLM analyst is initially a task classifier and narrator, not a router. Do not build routing scaffolding (lifecycle state machines, 11 recommendation types) for evidence that won't exist yet.

### 3.4 Adapters will be fat, not thin

Agent session formats are undocumented, unstable, lossy (streaming deltas, inconsistent cache reporting, per-message-only usage), and semantically incommensurable (Claude Code "subagents" ≠ OpenCode "agents" ≠ Codex "threads"). The adapter layer will be the **hardest, highest-maintenance component**, in permanent format-churn maintenance.

Design implications:

- **Store raw events; treat derived tables as a cache.** When an adapter improves, re-parse old sessions without data loss. This is the single most valuable architectural decision available right now.
- **Conformance fixtures** — anonymized golden sessions per agent, checked into the repo, as the drift test harness.
- Budget **format-churn maintenance** as the project's permanent tax — state it in the roadmap so nobody is surprised.
- Consider pushing an **improved session export format upstream into OpenCode** — a stable export beats scraping a volatile internal format. **Confirmed: pursue, but do not block the MVP** (Review Q5).

### 3.5 The canonical event hierarchy is over-specified

Session → Task → Agent execution → Model invocation presents **"Task" as observed** — but no coding agent emits a clean task entity. Task decomposition is *inference*; by the project's own provenance discipline it must be a **derived, inferred** construct, not a first-class structural entity. Same for several of the 22 proposed event types.

Recommendations:

- Replace the bespoke hierarchy with a **trace/span model** (parent/child spans + attributes + capability flags).
- Let "tasks" be an **inferred view** layered on top, with provenance labels.
- **No event type enters the canonical model until at least two real adapters have produced it.**
- Make a conscious, documented decision about **OpenTelemetry GenAI semantic conventions** (resolved: hybrid — Review Q2) rather than building a private taxonomy from imagination.

### 3.6 One embedded piece of folklore contradicts the doc's own principles

The "same-model self-review" flag embeds an unvalidated assumption into a supposedly evidence-based engine — exactly the "benchmark-adjacent folklore" the doc opposes. Either gather evidence for it in your own data, or keep it out of the recommendation layer. Evidence-based systems rot one folkloric rule at a time.

### 3.7 Cost estimation is a bigger liability than admitted

- Don't build a pricing catalog — **consume LiteLLM's open pricing dataset** (already maintained, versioned) and layer provenance on top.
- **The zero-cost trap:** for local/BYOK users (Ollama + GLM etc.) marginal cost is literally zero; the value proposition must shift to behavior, time, waste, and outcome. Value differs by segment: subscription users ("am I hitting rate limits / getting value?") vs API-billed vs local. Wedge confirmed as microscope (Review Q3); segment messaging to be refined during v0.1 dogfooding.
- Redaction is genuinely hard, not a regex pass: code excerpts can contain PII and customer data, not just API keys. Redaction-at-egress with a manifest ("here is exactly what will be sent") is the right design — but don't trust it to be complete, and market it accordingly.

### 3.8 The original MVP (13 checkmarks) was too big

See Part IV for the corrected MVP.

---

# Part II — Architecture

## 4. Schema decision space

| Option | Pros | Cons |
|---|---|---|
| **Bespoke schema** | Full control; coding-agent-native | Duplicates an industry effort; third-party adapters must learn your schema; you own evolution alone |
| **Pure OTel GenAI semconv** | Free ecosystem (collector, Grafana/Jaeger); agents already emit it; schema maintained by others | GenAI semconv is LLM-call-centric — no tools/edits/tests/outcomes; coding-agent semantics are your actual value |
| **Hybrid (recommended)** | OTel semconv as base vocabulary for model calls; **own extension events for coding-agent semantics** (tool_call, file_edit, test_run, human_intervention, outcome); OTLP as optional input transport | Must keep one eye on upstream semconv drift |

The hybrid keeps interop (a future `agentlens otlp` receiver makes any OTel-emitting agent work for free) while innovating where the gap actually is: coding-agent-native semantics and outcomes. Design the model **on paper against 2–3 real session formats before shipping any adapter**, even though v0.1 ships with only one — this avoids building "OpenCode's schema with a veneer."

## 5. Capture strategy

| Strategy | Richness | Fragility | Friction | Verdict |
|---|---|---|---|---|
| Post-hoc file import | Medium | High (format churn) | Zero | **MVP** |
| Live file tail (`watch`) | Medium+ | High | Zero | v0.2–0.3 |
| Agent hooks / event subscription | High | Medium | Per-agent work | v0.4+, contribute upstream where possible |
| **Model-API proxy / interposition** | Highest | Highest; breaks local setups; runtime liability | Requires trust | **Never** |

**Founding rule (confirmed — Review Q4): stay out of the runtime path.** AgentLens observes agents; it does not need to sit in the runtime path. Proxying model calls is the highest-maintenance, highest-trust-requirement architecture in this space, kills local/Ollama setups, and makes AgentLens a chokepoint for the thing it observes. The strongest long-term form of "routing" is a **policy generator** — AgentLens emits a config suggestion ("route docs to Flash in opencode.json") and the agent's own machinery executes it. **Advisor, never proxy.** This one decision prevents the project's most likely way to die.

## 6. Internal structure

Replace the derived-first design with an immutable event log at the base:

```text
Raw events (immutable, per-agent native, schema-versioned, unknown fields preserved)
    ↓  re-parseable at any time
Normalized span model (parent/child, attributes, capability flags)
    ↓
Derived views (metrics, trees, task timelines) — all rebuildable
    ↓
Inference cache (task labels, explanations) — model+prompt version recorded, never load-bearing
```

This survives format churn (re-parse old sessions when adapters improve), makes "Task" honestly inferred, and makes the LLM layer cheap to re-run when prompts change. Add a **session completeness score** ("this report is based on 60% of calls having usage data") so every report honestly self-grades.

## 7. Language decision — Rust vs Go — **resolved: Go**

TypeScript has been **excluded by the project owners**. The remaining candidates are Rust and Go. Both are viable; here is the project-specific comparison:

| Factor | Go | Rust | Weight for this project |
|---|---|---|---|
| Iteration speed during schema churn (the dominant early cost) | Fast compiles, simple language, cheap refactors | Slower compiles, more ceremony, richer types to redesign | **High — Go** |
| Fit for actual workload (JSONL parsing, SQLite, tables, terminal output — all I/O-bound) | Fully sufficient | More than sufficient | Neutral — no perf need |
| Event-model rigor | Enums + interfaces; exhaustiveness is runtime discipline | `serde` tagged enums; compile-time exhaustiveness checking | **Medium — Rust** |
| Distribution | Single static binary; trivial cross-compile | Single binary; cross-compile fine (cross / cargo-zigbuild) | Tie |
| SQLite access | `modernc.org/sqlite` (pure Go, no CGO) or `mattn/go-sqlite3` | `rusqlite` / `sqlx` | Tie |
| CLI ecosystem | Cobra + Charm (lipgloss/glow) | clap + ratatui | Tie |
| Contributor pool for community adapters | Broader for CLI/data tooling | Narrower | **Medium — Go** |
| Ramp-up velocity for a 2-person team | Days | Weeks+ | **High — Go** |
| Future embeddable core / WASM dashboard | Possible but awkward | First-class | **Low now — Rust** (only matters much later) |
| Safety on untrusted session input | Memory-safe (GC) | Memory-safe + fearless parsing | Tie in practice |

### Decision: **Go** (confirmed — Review Q1)

**Why:** the dominant cost of the next 6–12 months is **schema and format churn** — the data model will change weekly, adapters will be rewritten repeatedly, and views will be re-derived. Go maximizes iteration velocity on exactly that axis, covers 100% of the performance envelope this project will ever need (the workload is I/O-bound data plumbing), gives single-binary distribution with trivial cross-compilation (pure-Go SQLite, no CGO), and has the broader contributor pool for the community-adapter ecosystem the open-source strategy wants.

Rust remains a valid future option if a concrete technical reason appears (embeddable core, WASM, compile-time schema exhaustiveness as a hard requirement). The language decision is reversible; the data model is not.

## 8. Stack verdicts

- **SQLite: correct**, with the raw-event design. WAL mode, JSON columns for raw events, SQL views for metrics. DuckDB is tempting for analytics but adds a second engine for no gain at this scale. Postgres only if a team server ever exists (it shouldn't, for a long time).
- **CLI-first: correct.** Then a local read-only web UI served from the same binary (`agentlens ui`) — session trees and replay are inherently graphical, and web views produce shareable screenshots, which is how these tools grow. Skip a TUI (fun, time-sink, unshareable) and Electron (always). Full UI strategy: Part V.
- **LLM integration: keep the `Analyst` interface** from the original doc, but the default implementation is `NoOpAnalyst`. Semantic analysis lives behind an explicit `--explain` flag with an egress manifest. Record model+prompt version with every cached inference.
- **Pricing: consume the LiteLLM open pricing dataset**, label all derived costs as estimated, prefer provider-reported cost when present.

---

# Part III — Product

## 9. Positioning: the flight recorder

The strongest version of the idea is a local, open-source **flight recorder for coding agents**: it shows what your agent did, what it cost in time and money, whether the work verified, and — when you ask — explains it in plain language; over time it learns your patterns and nudges you toward better setups.

The flight-recorder framing bounds the product cleanly: **it observes, replays, and explains; it never flies the plane.** That is not a limitation — it is the positioning that makes AgentLens trustworthy enough to be allowed near the cockpit.

Note the ordering implication: the session tree/replay is not one feature among many — **inspection is the flagship**. Dashboards are commoditized; "why did my agent do that?" drill-down in context is not, and developers already manually scroll JSONL files for exactly this.

## 10. Metrics: what matters vs noise

The v1 "health panel" — keep it to ~6 metrics:

1. **Cached vs. uncached input ratio** — the single best context-strategy health metric, and directly = money.
2. **Cost per session/turn** (estimated, labeled, LiteLLM-sourced).
3. **Wall-clock vs. sum-of-spans** — parallelism health; critical-path section of the original doc is correct.
4. **Retry/repair loops + trigger cause** (test failure vs. lint vs. human nudge).
5. **Human intervention count/type** — the strongest quality signal actually collectable.
6. **Final status**: tests green in-session? diff produced? session ended in commit?

**Noise for v1** (drilldown only, not the panel): raw per-message tokens (aggregate only), tokens-per-skill (capability-dependent, unreliable across agents), subagent counts without outcomes, reasoning tokens beyond display, and most fine-grained metric variants from §13 of the original doc.

## 11. Multi-agent support without coupling

- **Structure-neutral core**: spans + attributes + capability flags. Family-specific semantics are *labeled attributes*, never first-class core entities.
- **Two adapter axes**: *capture* (file / live / OTLP / hook) and *semantic mapping* — separable, so third parties can ship either.
- **Preserve unknown fields** (raw passthrough) so future re-parses recover data you didn't understand at first.
- Accept honestly: **cross-agent session comparison is partly an illusion** — agents differ in workflow defaults, so the agent, not just the model, is confounded. Comparison within an agent is honest; comparison across agents needs caveats.
- Multi-agent support is correct as a *long-term moat* (your floor if any vendor ships excellent built-in observability) but as a *v0 goal* it triples the hardest work for little value. Design the schema against 2–3 formats on paper; ship one adapter deep; make the second adapter the schema-validation milestone. **Confirmed: second adapter = Claude Code** (Review Q7).

## 12. Optimization & outcome measurement

- **Observed** = timestamps, durations, per-call usage, model ids, tool calls, messages, edits, in-session test results.
- **Inferred** = task decomposition, intent, classification, root cause, outcome quality, "waste". Inference runs offline over immutable telemetry, is cached with model+prompt provenance, and is never load-bearing.
- **Uncertainty/provenance**: field-level `{value, provenance: exact|derived|estimated|inferred, source_ref}` **plus** derived-entity provenance (inferred "tasks" are labeled with the inference run that made them) **plus** the session completeness score.
- Experiments v1 = **conventions + templates + `agentlens label`**, not software.
- How does a cheaper model prove equivalence? Only via (a) controlled paired experiments on *new* tasks in the same class (never "re-runs" — context drifts), or (b) historical clustering with honest n. Both need outcome tiers (§3.2). Never replay identical tasks.

## 13. Model routing stance

- **Recommendation-based with human application, from day one through the end state. Never interposed.**
- The original doc's maturity ladder (Stage 1 human → 2 recommend → 3 auto low-risk → 4 fully automated) is right in spirit, but **cap at Stage 2** for the foreseeable life of the project. Stages 3–4 reframe as *policy generation consumed by the agent's own router*.
- Routing evidence arrives only after task classification + outcome ground truth + months of history (§3.3). Until then, the honest product is smoke detection + inspection.

---

# Part IV — MVP & Roadmap

## 14. MVP — updated scope (answers the attribution question)

**Owner's question (paraphrased):** the tool must not only say "what happened and how many tokens/time" — it must give details: *which agents and which models were used, how much token/time each took*, and even task-level attribution — e.g., an Implementer agent had 5 tasks, used GLM-5.3 for 3, DeepSeek for 1, Qwen for 1; the tool must detect and report each phase.

**Answer: this is largely MVP, and the rest is Phase 2 — nothing is dropped.** The boundary is not "feature vs. later feature"; it is **observed vs. inferred**:

| Breakdown level | Example | Source | Phase |
|---|---|---|---|
| Session totals | 47m, 92k tok, $1.24 est. | observed | **v0.1** |
| Per-turn (per user prompt) | Turn 2 took 12m / 30k tok | observed | **v0.1** |
| Per-agent / per-subagent | Explorer 9.4k, Implementer 38k | observed | **v0.1** |
| Per-model, per agent and overall | Implementer: GLM-5.3 24k, DeepSeek 6.3k, Qwen 4k | observed | **v0.1** |
| Per-model-call | call #17: GLM-5.3, 2.1k in / 300 out, 14s, cache hit | observed | **v0.1** |
| Per-tool | Bash: 61 calls, 8m | observed | **v0.1** |
| Per-task **when the agent marks tasks** (todo/plan tool calls are observable events) | Task 3 "add retry logic": GLM-5.3, 5k, 90s | observed | **v0.1** (best-effort) |
| Per-task **semantic segmentation** when unmarked | Implementer's 5 implicit tasks, with model per task | **inferred** | **v0.2** — labeled `[inferred]`, never shown as observed |
| Per-task-type across sessions | "docs tasks → Flash is fine" | inferred + history | v3+ |

Key points:

1. **Per-agent, per-model, per-tool, per-turn attribution is deterministic** — model identity and usage are recorded per message/call in OpenCode and Claude Code transcripts. It is squarely MVP.
2. **Task-level attribution has two cases.** If the agent used a todo/plan tool, task boundaries are *observed* → MVP (best-effort). If tasks are implicit inside one prompt, boundaries must be *inferred* (heuristic segmentation + optional LLM classification) → v0.2.
3. **The raw-store design (Part II §6) makes the v0.2 layer retroactive** — inferred task decomposition can be applied to sessions analyzed under v0.1, with no data loss. This is precisely why the raw-event store is the highest-leverage architectural decision.
4. Your exact example (Implementer, 5 tasks, GLM-5.3×3 / DeepSeek×1 / Qwen×1) renders **fully from v0.2**, and **partially in v0.1** (per-model totals under the Implementer, exact per-call attribution, and per-task detail whenever task markers exist).

Illustrative output (v0.1 + v0.2 task layer):

```text
$ agentlens last

SESSION ses_8f3a — opencode — 47m — est. $1.24 [pricing: estimated]
completeness: usage reported for 94% of model calls

Turn 1  "implement the 5 payment endpoints"              19m · 61k tok
  ├─ Explorer      glm-5.3            6 calls   9.4k/0.8k    2m10s
  ├─ Implementer   (task breakdown below)      38.1k/6.2k  14m05s
  │    ├─ Task 1  payments API scaffold     glm-5.3      8.1k   3m12s
  │    ├─ Task 2  retry logic              glm-5.3      6.4k   2m41s
  │    ├─ Task 3  idempotency keys         deepseek-v3  6.3k   2m40s
  │    ├─ Task 4  webhook signature check   qwen3        4.0k   1m20s
  │    └─ Task 5  integration tests         glm-5.3      5.2k   2m05s
  │    [task boundaries: inferred — segmenter v0.2, confidence 0.8]
  └─ Reviewer      glm-5.3            3 calls  12.0k/1.1k    3m40s

BY AGENT            calls      in/out tok        time     est.cost
  Explorer             6       9.4k / 0.8k      2m10s      $0.09
  Implementer          24     38.1k / 6.2k     14m05s      $0.71
  Reviewer              3      12.0k / 1.1k     3m40s      $0.22

BY MODEL
  glm-5.3              27      49.2k / 7.1k     15m30s      $0.88
  deepseek-v3           4       6.3k / 0.9k      2m40s      $0.12
  qwen3                 2       4.0k / 0.6k      1m20s      $0.05
```

### MVP scope (v0.1)

> *What happened in this OpenCode session, and where did the time/tokens/cost go — per agent, per model, per tool, per turn?*

```text
1. Discover + read local OpenCode sessions (post-hoc, zero friction)
2. Parse → raw event store (SQLite), minimal neutral schema, unknown fields preserved
3. agentlens last / agentlens session <id>:
   - tree outline (turns → agents → model calls → tools)
   - tables: tokens/time/cost by turn / agent / model / tool
   - per-task detail where task markers are observed
   - provenance labels on everything derived
4. agentlens share <id>  (redacted JSON export)
```

## 15. Roadmap (evidence-gated, not date-gated)

| Version | Answers | Contents |
|---|---|---|
| **v0.1** | "What happened?" | MVP above |
| **v0.2** | "What's inefficient?" | 4 deterministic smells (cache ratio, repeated context, retry loops, compaction frequency) · second adapter (schema validation milestone) · **inferred task decomposition (retroactive)** |
| **v0.3** | "How does it compare?" | `agentlens compare` · `agentlens label` (human ground truth) · historical drilldowns |
| **v0.4** | "Explain it to me" | `--explain` (LLM analyst, opt-in, local model default, egress manifest) |
| **v0.5** | "Show me" | `agentlens ui` — local read-only web UI from the same binary: session list · session tree (flagship) · timeline / critical path · breakdown charts (Part V) |
| **v1.0** | stability | schema freeze + adapter SDK + docs |
| later, evidence-gated | — | OTLP receiver · routing advisor (policy generator) · experiment tooling · CI reporting |

## 16. Deferred and never

**Explicitly deferred** (decide again only on evidence): model routing in any form · experiment-framework software · CI/PR integration · team server · task classification beyond smoke thresholds · aggregate analytics beyond per-user local.

**Never (founding constraints — confirmed, Review Q4):** no model-call proxy / runtime interposition · never execute commands from session data · never act on the user's workflow uninvited · never gate CI on token counts · hosted functionality never required for local observability · never fabricate missing telemetry. Wording note: the review doc's "never require cloud connectivity" should be tightened to "the deterministic core works fully offline; any network feature is explicitly invoked and opt-in" (see §26C).

---

# Part V — UI & Dashboard Strategy

## 17. Why the UI is a first-class product surface, not a later add-on

For *this* product, the UI is not optional and not merely cosmetic:

- **Session forensics is inherently graphical.** Trees (turn → agent → call → tool), timelines (critical path, parallelism), stacked token/cost bars per turn — these are all structures humans parse visually, not as terminal tables.
- **Inspection is the flagship** (§9) and the flagship deserves better than ASCII art. The CLI remains the fastest path ("what happened just now?"); the UI is where *understanding* happens.
- **Screenshots are the growth loop.** A shareable PNG of "look what my agent did" travels (social, blog posts, issues) in a way text output never will. `agentlens share` (CLI) and `agentlens ui` (visual) are two formats of the same redaction pipeline.
- **The UI is a *view*, never a source of truth.** Every number rendered must come from the same SQLite store and carry the same provenance labels as the CLI. No UI-only computations — this keeps CLI and UI consistent by construction.

## 18. Sequencing: CLI proves the model, UI visualizes it

| Rule | Rationale |
|---|---|
| **v0.1 ships CLI-only.** | The schema is the product's spine; the CLI forces it to be right. A UI built on a churning schema is rewritten three times. |
| **UI ships at v0.5, after the `--explain` layer** | By then the schema, metrics, smells, comparison, and explanation formats exist — the UI has something real to visualize. |
| **The UI must be droppable.** | `agentlens ui` serves a read-only SPA from the same binary (embedded via `go:embed`). No server, no auth, no database migrations — lose the UI, keep the tool. |
| **Same data, same labels.** | `[inferred]`, `[estimated]`, completeness % render identically in CLI and UI. |

This is *not* "we don't know the UI yet" — it's a deliberate order: **prove the model in text, visualize it once it's stable.** Timing confirmed by the owners: the UI is not a necessity early; v0.5 sequencing stands (see D12).

## 19. View set for the first UI (v0.5) — small and deep, not a dashboard grid

The first UI is a **session inspector**, not a generic dashboard. Four views only:

### 19.1 Session list
- Recent sessions: agent, project, duration, tokens, est. cost, final status, completeness badge.
- Sort/filter by date, agent, project, label (once `label` exists, v0.3).

### 19.2 Session tree — the flagship view
- Collapsible tree matching the CLI outline: turns → agents → model calls → tool calls, with tokens/time/cost per node (inline bars).
- Click any node → side panel: full detail (messages, diffs, command output, per-call usage).
- `[inferred]` nodes (task decomposition, v0.2) get a distinct visual style (dashed border) + confidence — **inferred data must never look identical to observed data.**
- This is the answer to "why did my agent do that?" in one screen.

### 19.3 Timeline / critical path
- Horizontal time-axis view: rows = agents/subagents, bars = calls, overlaid markers = human interventions, retries, compactions.
- Makes parallelism and idle time visible — the thing `wall_clock vs sum-of-spans` tries to say in numbers (§10).
- Later (v0.3+): scrub-through replay (§22) reuses this exact canvas.

### 19.4 Breakdown charts
- Treemap / stacked bars: tokens, time, cost — sliceable by agent / model / tool / turn / task.
- Chart interactions must stay **drill-down oriented** (click a segment → jump to those spans in the tree), not passive pie-in-the-sky.

**Explicitly postponed:** trends over sessions, historical aggregates, model-comparison charts, experiment dashboards, anything that smells like "team analytics". Those are analytics products; until real history exists (and per §3.3 it won't for months), they'd be empty canvases with fake purpose.

## 20. UI design language (the "lens" identity)

- **Data-ink over decoration.** This is a developer forensics tool; think profiler (Chrome DevTools Performance tab, flamegraphs), not marketing dashboard. Dark default. Monospace for identifiers.
- **Provenance is the visual identity.** The four provenance states (observed / inferred / estimated / unavailable) get a consistent, learnable treatment across every view — color, border, badge — so honesty about data quality is literally the brand.
- **No vanity metrics.** Every chart answers a question a developer actually asked; if a view can't be phrased as a question ("where did the time go?"), it doesn't ship.

## 21. Frontend stack decision space — **resolved: React**

| Option | Pros | Cons | Verdict |
|---|---|---|---|
| **React + Vite, embedded via `go:embed`** | Largest ecosystem; huge charting selection; most contributors know it; embeds cleanly into the Go binary | Node toolchain in the repo; heavier | **Chosen (owner)** |
| SvelteKit | Lighter, simpler, fast; pleasant for small SPAs | Smaller ecosystem; fewer chart primitives | Rejected |
| Vanilla TS + Canvas/SVG (no framework) | Zero deps, tiny bundle, total control | You rebuild trees/timelines/pan-zoom by hand — the flagship views are exactly the hard parts | Rejected |
| HTMX / server-rendered | No JS app at all | Interactive trees/timelines/scrubbing are the product — server-rendering fights the product | Rejected |
| Tauri desktop app | Native feel | Distribution burden, separate runtime, no gain for a local read-only UI | Rejected |

**Decision (owner-confirmed): React + Vite**, embedded in the Go binary via `go:embed`. Charting: **ECharts** for standard charts; hand-rolled SVG/D3 **only** for the two custom views (session tree, timeline) that no library does well. Don't build the whole UI in D3.

Timing (owner-confirmed): the UI is **not** required early — it stays at **v0.5**, after the schema (v0.1), smells + second adapter + task inference (v0.2), comparison + labels (v0.3), and `--explain` (v0.4) have stabilized the data model it visualizes. This sequencing costs nothing and avoids the 2–3 UI rewrites a UI-on-churning-schema would cause.

## 22. Longer-range UI (post-v1, strictly evidence-gated)

- **Session replay / scrubber** — step through the timeline with state reconstruction. High wow-value, high effort; only after the timeline view proves itself.
- **Diff explorer** — browse file edits per turn with inline diffs (git attribution view, §B.4).
- **Experiments view** — only when the experiment conventions (v3+) produce data worth charting.
- **Comparison mode** — side-by-side trees for `agentlens compare` results.
- Each of these is *earned* by usage of the previous UI layer, not planned by date.

---

# Part VI — Building AgentLens: the development agent team

The goal: a small, repo-versioned, **tool-neutral** agent team usable from both OpenCode and Claude Code (and portable to future agents). Six agents, no more.

## 23. The six agents

### 23.1 Architect
- **Mandate:** owns the canonical schema, ADRs, the adapter contract, and provenance rules. Gatekeeper: no change to the schema or adapter contract merges without its review.
- **Invoke for:** design questions, schema changes, ADR drafting, "should we add X" questions.
- **Model tier:** strongest available.
- **Prompt guardrails:** demands provenance semantics for every field; enforces "no event type without two producing adapters"; prefers deleting abstractions over adding; writes the ADR before code for cross-cutting changes.

### 23.2 Format Investigator
- **Mandate:** reverse-engineers session formats (OpenCode first, Claude Code next); produces a **format dossier** per agent — file locations, record types, usage/model-identity fields, quirks, drift history — and builds anonymized golden fixtures. This is the specialist for the project's hardest, highest-maintenance part.
- **Invoke for:** before *any* adapter work on a format.
- **Model tier:** strong reasoning (format archaeology is fiddly).
- **Prompt guardrails:** never guesses — verifies against real session files; records "unknown" explicitly; every dossier claim links to a fixture.

### 23.3 Implementer
- **Mandate:** feature and adapter code from plans; small verified steps; follows repo conventions.
- **Model tier:** strong mid-tier.
- **Prompt guardrails:** no agent-specific parsing outside `adapters/`; tests against fixtures; never invents telemetry; preserves raw passthrough.

### 23.4 Reviewer
- **Mandate:** defect-focused review; enforces the project invariant checklist (provenance labels present; adapter isolation; untrusted-input handling; no execution of session content; no fake precision; no schema change without Architect sign-off).
- **Model tier:** strongest — and **deliberately different from the Implementer's model** when feasible (this doubles as the first data point on the "same-model self-review" folklore question).
- **Prompt guardrails:** findings by severity with evidence; distinguishes defect vs. preference; never approves schema changes without the Architect.

### 23.5 Test Engineer
- **Mandate:** conformance fixtures, golden-session regression harness, normalizer table tests, drift-alarm tests.
- **Model tier:** mid.
- **Prompt guardrails:** every fixture anonymized; fixture provenance recorded; tests fail loudly on schema drift.

### 23.6 Documenter
- **Mandate:** schema reference, adapter guide, ADR polishing, dossier publishing, README.
- **Model tier:** cheap.
- **Prompt guardrails:** docs only from code + dossiers; no speculative documentation.

## 23.7 Workflow recipes

```text
New adapter:   Investigator (dossier) → Architect (contract review) → Implementer → Test Engineer (fixtures) → Reviewer
Schema change:  Architect (ADR) → Implementer → Reviewer → Documenter
Feature:       plan → Implementer → Reviewer → Test Engineer
```

## 23.8 Portability layout (OpenCode + Claude Code compatible)

```text
repo/
  agents/                    ← source of truth (tool-neutral markdown)
    architect.md
    format-investigator.md
    implementer.md
    reviewer.md
    test-engineer.md
    documenter.md
  .claude/agents/            ← thin copies for Claude Code
  .opencode/agent/           ← thin copies for OpenCode
  scripts/sync-agents.sh      ← copies/syncs agents/ into both tool dirs
  AGENTS.md / CLAUDE.md      ← shared engineering rules (tool conventions map)
```

- Frontmatter kept to the intersection both tools understand (`name`, `description`, `model`, tools hints — check each tool's docs for exact supported fields).
- Prompt bodies written **tool-neutral** — no tool-specific tool names inside bodies; tool conventions live in `AGENTS.md` / `CLAUDE.md`.
- Agents are markdown files in the repo: versioned, reviewable, diffable artifacts.

Example frontmatter (Architect):

```yaml
---
name: architect
description: Owns the AgentLens schema and architecture. Reviews all design decisions and ADRs. Use for design, schema, and cross-cutting questions.
model: strongest
---
[prompt body — tool-neutral]
```

## 23.9 Dogfooding note

Once AgentLens v0.1 exists, point it at the sessions these six agents produce. The project becomes its own first user, and the agent team becomes the first measurable workflow — including the first routing experiment (does the Reviewer on the flagship model catch more real defects than on mid-tier?). This closes the loop with the original doc's rule: *"treat the project itself as an AI-engineering laboratory."*

---

# Part VII — Decision Log

> **Status update (2026-09-14):** the follow-up document `AgentLens_FINAL_ARCHITECTURE_REVIEW.md` answered open questions Q1–Q12 and introduced three new confirmed decisions (Q13 Trace-root, Q14 content opt-in, Q15 early low-confidence recommendations). The owner subsequently resolved the remaining clarifications C1–C3 and accepted the Q4 wording fix. **All open items are now resolved.** The decision log below is the consolidated, current state of all decisions.

## 25. Resolved decisions (consolidated)

| # | Decision | Final state | Source |
|---|---|---|---|
| D1 | TypeScript excluded as implementation language | **Resolved** | Owner |
| D2 | **Language: Go** (schema churn + I/O workload dominate; Rust advantages unneeded for MVP; decision reversible) | **Resolved** | Review Q1 |
| D3 | Per-agent / per-model / per-tool / per-turn attribution is MVP scope | **Resolved** | Owner |
| D4 | Per-task semantic segmentation = v0.2, inferred + labeled, retroactive via raw store | **Resolved** | Owner |
| D5 | Raw-event store with rebuildable derived views; unknown fields preserved; all derived info rebuildable from raw | **Resolved** | Architecture + Review §6 |
| D6 | **Founding constraints confirmed** — no runtime proxy/interposition, never execute session content, never fabricate telemetry, never gate CI on token counts, never require cloud/hosted | **Resolved** | Review Q4 |
| D7 | Cost estimation via LiteLLM dataset, provenance-labeled | **Adopted** | Architecture |
| D8 | Six-agent development team accepted (Architect, Format Investigator, Implementer, Reviewer, Test Engineer, Documenter) | **Resolved** | Review §7 |
| D9 | UI confirmed as first-class product surface | **Resolved** | Owner |
| D10 | UI sequencing: CLI-only until v0.5, then local read-only web UI (embedded, no server/auth) | **Resolved** | Owner + Review Q11 |
| D11 | First UI = 4-view session inspector (list, tree, timeline, breakdown); aggregate dashboards postponed | **Adopted** | Architecture |
| D12 | UI timing: v0.5 confirmed; early shipping not a priority | **Resolved** | Owner |
| D13 | **Frontend: React + Vite + ECharts**, embedded in binary; UI stays v0.5 | **Resolved** | Review Q11 |
| D14 | **UI visual identity: dark profiler / developer-forensics style** (DevTools/flamegraph mental model, provenance states as visual language) | **Resolved** | Review Q12 |
| D15 | **Schema: hybrid OTel** — OTel GenAI vocabulary as base + AgentLens extensions for coding-agent semantics; not an OTel Collector, provider-neutral | **Resolved** | Review Q2, §5 |
| D16 | **Primary wedge: microscope** (session inspection is the product; tokens/cost/time calculator is the hook) | **Resolved** | Review Q3 |
| D17 | **OpenCode upstream export format: pursue, but do not block MVP** — consume official surfaces first | **Resolved** | Review Q5 |
| D18 | **Outcome claims: Tier 1 (observed in-session signals) + `agentlens label`**; no strong model-equivalence claims until evidence exists | **Resolved** | Review Q6 |
| D19 | **Second adapter: Claude Code** (first schema-validation milestone) | **Resolved** | Review Q7 |
| D20 | **Name: AgentLens** (`agentlens` CLI verb) | **Resolved** | Review Q8 |
| D21 | **License: Apache-2.0** | **Resolved** | Review Q9 |
| D22 | **Analyst: NoOp by default; `agentlens explain` opt-in; Ollama first provider; every result records provider/model/prompt-version provenance** | **Resolved** | Review Q10 |
| D23 | **Canonical structural root = Trace** (not Task, not agent-native session); agent-native session is source metadata; inferred task is derived view | **Resolved** | Review Q13 |
| D24 | **Content capture is opt-in** (metadata-first default; `metadata-only` / `content-local` / `redacted-export` modes; no implicit cloud transfer) | **Resolved** | Review Q14 |
| D25 | **Recommendations may appear early with explicit LOW confidence**; evidence-backed recommendations come later | **Resolved** | Review Q15 |
| D26 | **Role → default model tier, not fixed model binding** (the team itself becomes the first AI-workflow experiment) | **Resolved** | Review §9 |
| D27 | **C1 — Ollama confirmed as the AI runtime**: Ollama installed; dogfooding uses Ollama Cloud models (`glm-5.3:cloud`, `deepseek-v4-pro:cloud`) rather than local inference. Deterministic core stays LLM-free; `explain` is exercised with Ollama during development | **Resolved** | Owner (C1) |
| D28 | **C2 — Content-local storage: references, not copies.** AgentLens does NOT store full prompts/responses in its SQLite by default; it stores hashes/references and **re-reads original agent session files on demand**. A future explicit snapshot/export mode may be added when reproducibility requires it | **Resolved** | Owner (C2) |
| D29 | **C3 — Roadmap v0.1 → v0.5 sequence confirmed as-is.** UI, LLM explanation, routing, experiments, and broader analytics stay out of the MVP | **Resolved** | Owner (C3) |
| D30 | **Q4 wording fix adopted:** "The deterministic core works fully offline; any network-dependent feature is explicitly invoked and opt-in." (Better than "never require cloud connectivity" — `explain` via Ollama Cloud and pricing updates are legitimate optional network features) | **Resolved** | Owner |

## 26. Remaining items — all resolved (2026-09-14)

The review document resolved Q1–Q15; the owner has since answered every remaining item. Kept here for the record, with final resolutions:

### A. Implementation-level decisions (adopted with recommended defaults)

| # | Item | Resolution |
|---|---|---|
| R1 | **Redaction for `agentlens share`** | v0.1 ships regex-based known-secret patterns (AWS/GCP/OpenAI/Anthropic keys, JWTs, `.env` names, authorization headers) + visible "redacted: N items" manifest + explicit not-a-guarantee framing. Smarter detection (entropy scanning, LLM-assisted PII) is v0.3+. |
| R2 | **LiteLLM pricing data updates** | Vendored snapshot in the repo + `agentlens pricing update` (manual, explicit); snapshot version recorded in every cost estimate's provenance. |
| R3 | **Go SQLite driver** | `modernc.org/sqlite` (pure Go, zero-CGO cross-compilation); revisit only on demonstrated need. |
| R4 | **Session discovery scope** | Global by default (`~/.agentlens/` index), per-project views via `agentlens sessions --project .`. |
| R5 | **`agentlens label` schema** | Labels table reserved in the v0.1 schema (empty), used in v0.3 — avoids a later migration. |

### B. Owner clarifications — answered

| # | Question | Answer |
|---|---|---|
| C1 | Does the owner actually run Ollama, and with which models? | **Yes** — Ollama installed; dogfooding via Ollama Cloud models (`glm-5.3:cloud`, `deepseek-v4-pro:cloud`). `explain` is exercised during development (D27). |
| C2 | Store full prompts/responses in AgentLens SQLite, or re-read from agent session files on demand? | **Re-read from source on demand.** AgentLens stores references/metadata only; DB stays lean, no persistent duplication of sensitive content; explicit future snapshot/export mode allowed for reproducibility (D28). |
| C3 | Confirm the v0.1 → v0.5 roadmap sequence. | **Confirmed as-is.** Nothing gets pulled into the MVP prematurely (D29). |

### C. Wording fix — adopted

- The review doc's "never require cloud connectivity" is replaced by: **"The deterministic core works fully offline; any network-dependent feature is explicitly invoked and opt-in."** (`explain` via Ollama Cloud and pricing updates are legitimate optional network features.) The same fix should be applied in `AgentLens_FINAL_ARCHITECTURE_REVIEW.md` §2 Q4 (D30).

**Bottom line: nothing is left open. The next deliverable is the v0.1 implementation plan** (repo structure, schema, adapter contract, CLI commands, test strategy, implementation order) — per the review doc's own closing instruction: *do not start implementation until the v0.1 architecture is explicit.*

---

# Appendix — Changes to the original document

## A. What to remove from the current proposal

1. "Task" as a first-class canonical entity → derived/inferred view.
2. The 22-event taxonomy → grown from real adapter data, minimum viable set first.
3. Recommendation lifecycle state machine + 11 recommendation types → one generic `suggestion` record with evidence.
4. Experiment framework as software → conventions + templates + `label`.
5. CI/PR integration from the near roadmap.
6. Team server from any numbered version.
7. Routing stages 3–4 from the near-term roadmap → vision prose only.
8. Repo layout + stack decisions from the vision doc (downstream of undecided questions).
9. Same-model-review flag as a system-emitted signal, until evidenced in your own data.

## B. What to add

1. Raw-event store with re-parseable derived views.
2. Session completeness score.
3. `agentlens label` — human ground-truth bootstrap.
4. Git attribution view (which files/lines did this session touch).
5. `agentlens share` — redacted export, the growth loop.
6. `agentlens explain` — the flagship LLM feature, opt-in, local default (Ollama-first Analyst; NoOp by default; every result records provider/model/prompt-version provenance).
7. Conformance fixtures + schema versioning as first-class repo artifacts.
8. Explicit format-churn maintenance budget in the roadmap.
9. A documented decision on OTel alignment.
10. An explicit "never" list as a founding-constraints section.
11. UI & dashboard strategy (Part V): 4-view session inspector, provenance-as-visual-language, React + Vite embedded at v0.5.