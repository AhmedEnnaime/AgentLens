# AgentLens — Release Plan & Issue Breakdown

**Status:** Agreed baseline for delivery planning
**Date:** 2026-09-14
**Sources:** `AgentLens_Brainstorming_Report.md` (decisions D1–D30) · `AgentLens_FINAL_ARCHITECTURE_REVIEW.md` (confirmed checklist)

---

## How to read this document

- Each **release answers one question** from the product ladder. A release is done when that question is honestly answerable on real sessions.
- Each release is split into **issues**. An issue is a unit of work sized **~1–3 focused days**: small enough to review carefully, big enough to be meaningful. Nothing here is a detailed plan — **each issue gets its own mini-plan/architecture when work on it starts**, per our working agreement.
- Issues map to the six-agent development workflow (`Part VI` of the brainstorming report): adapter issues follow the Investigator → Architect → Implementer → Test → Review pipeline; schema issues require an ADR; features follow plan → Implementer → Reviewer → Test.
- **Every issue that touches parsing or normalization must ship with fixture tests.** No fixture, no merge.

### Release overview

| Release | Question it answers | Theme |
|---|---|---|
| **v0.1** | "What happened, and where did the resources go?" | Flight recorder for OpenCode |
| **v0.2** | "What's inefficient?" | Smells + second adapter + task inference |
| **v0.3** | "How does it compare?" | Labels, comparison, history |
| **v0.4** | "Explain it to me." | Opt-in LLM analyst + early recommendations |
| **v0.5** | "Show me." | Local read-only UI |
| **v1.0** | "Is it stable and shareable?" | Freeze, SDK, docs, distribution |
| **Post-1.0** | — | Evidence-gated backlog (not scheduled) |

---

# v0.1 — "What happened, and where did the resources go?"

**Goal:** run `agentlens last` on a real OpenCode session and get an accurate, provenance-labeled breakdown per turn / agent / model / tool — fully offline.

| ID | Issue | Scope | Done when |
|---|---|---|---|
| v0.1-01 | Repo bootstrap | Go module, `cmd/agentlens`, Apache-2.0 LICENSE, README stub, CI (build + test + lint), `.gitignore`, initial push to GitHub repo | Cloning the repo and `go build ./... && go test ./...` passes in CI |
| v0.1-02 | Agentic dev setup (agents + tool configs) | The six agent definitions in `agents/` (tool-neutral markdown); thin copies into `.opencode/agent/` and `.claude/agents/`; sync script; `AGENTS.md`/`CLAUDE.md` engineering rules; **`opencode.json` with auto permission mode** (schema-validated, model defaults, auto-allow edits/bash for frictionless agent-driven work, deny-list for destructive commands, instructions wiring, compaction settings); Claude Code project settings tuned the same way | All six agents load in both OpenCode and Claude Code; auto mode enabled; configs validate; sync script committed |
| v0.1-03 | OpenCode format dossier + fixtures | Format Investigator deliverable: reverse-engineer OpenCode session storage (locations, record types, usage/model fields, quirks); anonymized golden fixtures checked into repo | Dossier doc exists; every claim links to a fixture |
| v0.1-04 | Canonical model v1 (ADR-1) | Go types for Trace/Span/Event, provenance enum (`observed/derived/estimated/inferred/unavailable`), capability flags; ADR for hybrid OTel mapping (OTel GenAI vocabulary + AgentLens extensions) | Types reviewed by Architect; unit tests; ADR merged |
| v0.1-05 | Raw event store | SQLite schema: raw events (immutable, unknown fields preserved), traces, spans, labels table (reserved, empty — R5); migrations; ingest API; `modernc.org/sqlite` (R3) | Events persist immutably; re-ingest is idempotent; migrations run forward-only |
| v0.1-06 | Privacy modes + content references | Metadata-first default: store hashes/references to source session files, re-read content on demand (D28); `metadata-only` / `content-local` / `redacted-export` config | No full prompt/response content lands in AgentLens SQLite under default mode; re-read API returns content from source files |
| v0.1-07 | OpenCode adapter: discovery | Find + index OpenCode session files; global index in `~/.agentlens/` (R4); `--project` filtering | `agentlens sessions` lists real sessions from all projects |
| v0.1-08 | OpenCode adapter: parse → raw events | Parse session files per dossier into raw event records; tolerate format drift; preserve unknowns | All fixture sessions parse without error; unknown fields retained |
| v0.1-09 | OpenCode adapter: normalize → canonical spans | Map raw events to canonical spans: turns, agents, model calls, tool calls; per-call usage + model identity; capability declaration for OpenCode | Fixture conformance tests pass; spans carry per-call tokens/timings with provenance |
| v0.1-10 | Pricing + cost estimation | Vendored LiteLLM pricing snapshot (R2); `agentlens pricing update` (manual, explicit); cost = tokens × price, always labeled `[estimated]`, snapshot version in provenance | Costs computed for all fixture sessions; provenance shows pricing source + version |
| v0.1-11 | Metrics engine + completeness | Session totals; per-turn / per-agent / per-model / per-tool breakdowns; wall-clock vs sum-of-spans; completeness score (usage coverage %) | All breakdowns computed from spans; completeness rendered on every report |
| v0.1-12 | CLI: reports | `agentlens last`, `agentlens sessions`, `agentlens session <id>`: tree outline (turns → agents → calls → tools) + breakdown tables + node detail (on-demand content re-read) | Output matches the illustrative format in the brainstorming report §14 (minus inferred tasks) |
| v0.1-13 | Redaction + `agentlens share` | Regex-based known-secret redaction (R1: API keys, JWTs, `.env`, auth headers); "redacted: N items" manifest; not-a-guarantee framing; redacted JSON export | Exported fixtures contain no known-secret patterns; manifest lists what was touched |
| v0.1-14 | Conformance harness + dogfood pass | Fixture-driven regression suite wired into CI; run AgentLens on real working sessions (dogfooding), fix gaps found | CI fails on fixture drift; at least 5 real sessions analyzed correctly end-to-end |

**Exit criteria:** a developer with OpenCode can install, run `agentlens last`, and trust the numbers (completeness + provenance visible). No LLM, no UI, no network.

---

# v0.2 — "What's inefficient?"

**Goal:** deterministic workflow-smell detection over stored sessions, a second adapter proving the schema is agent-neutral, and inferred task decomposition (retroactive).

| ID | Issue | Scope | Done when |
|---|---|---|---|
| v0.2-01 | Derived-view rebuild command | `agentlens reprocess`: re-run normalization + derived views over existing raw events (adapter upgrades apply to old sessions) | Upgrading an adapter and reprocessing reproduces correct output on old fixtures |
| v0.2-02 | Smell: cache ratio | Cached vs uncached input ratio per session/turn; flag poor context-cache health | Detects the smell on crafted fixtures; clean sessions not flagged |
| v0.2-03 | Smell: repeated context | Detect repeated large context blocks across calls in a session | Repeated-context % computed and reported; fixture-verified |
| v0.2-04 | Smell: retry/repair loops | Detect retry loops; classify trigger cause (test failure / lint / human nudge) where observable | Loop count + causes reported; fixture-verified |
| v0.2-05 | Smell: compaction frequency | Detect frequent compactions as context-pressure signal | Frequency reported; fixture-verified |
| v0.2-06 | Failure & outcome event capture | Normalize observable Tier-1 outcome signals: in-session test/lint runs + results, human interrupts/edits | Outcome events present in canonical spans; capability-flagged |
| v0.2-07 | Smell report: `agentlens analyze` | Render the health panel: the ~6 metrics that matter (§10 of the report) + detected smells | One command shows health panel for any stored session |
| v0.2-08 | Claude Code format dossier + fixtures | Same deliverable as v0.1-03 for Claude Code | Dossier + anonymized fixtures merged |
| v0.2-09 | Claude Code adapter | Discovery + parse + normalize for Claude Code (the schema-validation milestone: no core changes allowed beyond labeled attributes) | Claude Code sessions produce equivalent reports; core model needed zero breaking changes (or ADR documents them) |
| v0.2-10 | Task inference (derived view) | Heuristic segmentation of unmarked tasks into an inferred task view; `[inferred]` labels + confidence; retroactive via reprocess | The 5-task/3-model example from the report renders with correct attribution and inferred labeling |
| v0.2-11 | Task view rendering | Integrate inferred tasks into tree/report output (CLI) | Reports show observed task markers as-is and inferred tasks distinctly |

**Exit criteria:** `agentlens analyze` finds real waste on real sessions; Claude Code works without core surgery; task-level attribution renders and is honestly labeled.

---

# v0.3 — "How does it compare?"

**Goal:** human ground truth, session comparison, and lightweight history — all local.

| ID | Issue | Scope | Done when |
|---|---|---|---|
| v0.3-01 | `agentlens label` | Human good/bad labels on sessions (uses the table reserved in v0.1); label provenance = human | Labels stored and shown in session list |
| v0.3-02 | `agentlens compare` | Side-by-side comparison of two sessions: cost/time/tokens/retries/outcome dimensions; honest caveats on cross-agent comparison | Compare renders correctly for two real sessions |
| v0.3-03 | Git attribution view | Which files/lines did this session actually touch (from observed edit events); per-session diff summary | Session report shows touched-files summary |
| v0.3-04 | Historical drilldowns | `agentlens stats`: local aggregates over all stored sessions (by project/model/agent/day); drilldown-oriented, not dashboards | Stats command answers "what are my most expensive patterns" from real data |
| v0.3-05 | Outcome panel | Tier-1 outcome summary per session (tests green? retries? interventions? diff produced?) feeding compare + stats | Outcome column appears in list/compare/stats |

**Exit criteria:** a developer can answer "is this workflow better than that one?" with labeled, outcome-aware evidence.

---

# v0.4 — "Explain it to me."

**Goal:** opt-in LLM analysis over deterministic evidence, with privacy posture and provenance intact, plus early explicitly-low-confidence recommendations.

| ID | Issue | Scope | Done when |
|---|---|---|---|
| v0.4-01 | Analyst interface + NoOp | `Analyst` abstraction, provider registry, NoOp default; deterministic core untouched | Default behavior unchanged; interface ADR merged |
| v0.4-02 | Ollama provider | OpenAI-compatible/Ollama provider (incl. cloud models `glm-5.3:cloud`, `deepseek-v4-pro:cloud`); explicit `--model` selection | `explain` works against the owner's Ollama setup |
| v0.4-03 | Egress manifest + privacy enforcement | Show exactly what will be sent before sending; enforce privacy modes for explain input; nothing implicit | Manifest renders; redacted/metadata-only modes honored |
| v0.4-04 | `agentlens explain` | Semantic explanation of a session from structured evidence (deterministic metrics + events); LLM never invents telemetry | Explanation cites observed numbers; clearly-separated interpretation |
| v0.4-05 | Inference cache + version provenance | Cache semantic results; record provider/model/prompt-version with every result (never load-bearing) | Re-running explain is cheap; results carry provenance |
| v0.4-06 | Early recommendations (LOW confidence) | Generic `suggestion` record with evidence + explicit confidence (D25); `agentlens recommendations`; smoke-based suggestions first | Suggestions render with confidence + evidence; no routing claims |

**Exit criteria:** `agentlens explain <session>` produces a useful plain-language narrative on real sessions, fully opt-in, provenance-clean.

---

# v0.5 — "Show me."

**Goal:** local read-only web UI from the same binary — the four-view session inspector. No server, no auth, droppable.

| ID | Issue | Scope | Done when |
|---|---|---|---|
| v0.5-01 | Read-only HTTP API | JSON endpoints over the SQLite store (list, session, spans, metrics, stats); same data as CLI | UI-ready endpoints serve fixture sessions |
| v0.5-02 | Frontend scaffold + embed | React + Vite + ECharts; `go:embed` into the binary; `agentlens ui` serves read-only SPA | Single binary serves the app locally |
| v0.5-03 | Theme + provenance visual language | Dark profiler/DevTools aesthetic; consistent treatment for observed/inferred/estimated/unavailable; completeness badges | Provenance states visually distinct across all views |
| v0.5-04 | Session list view | Recent sessions with agent, project, duration, tokens, cost, status, completeness; sort/filter | List renders real sessions |
| v0.5-05 | Session tree view (flagship) | Collapsible turns → agents → calls → tools; inline token/time/cost bars; node detail panel with on-demand content re-read; dashed styling for inferred nodes | Drill-down answers "why did my agent do that?" in one screen |
| v0.5-06 | Timeline / critical path | Time-axis view: rows per agent, bars per call, markers for interventions/retries/compactions | Parallelism and idle time visible at a glance |
| v0.5-07 | Breakdown charts | ECharts treemap/stacked bars sliceable by agent/model/tool/turn/task; click-through to tree | Every chart segment drills into spans |
| v0.5-08 | Share from UI | Redacted export/screenshot path reusing the redaction module | UI share output matches CLI `share` guarantees |

**Exit criteria:** the UI renders a real session end-to-end and produces a shareable, redacted artifact; CLI remains fully functional without the UI.

---

# v1.0 — "Is it stable and shareable?"

**Goal:** freeze what works, make third-party integration possible, and distribute properly.

| ID | Issue | Scope | Done when |
|---|---|---|---|
| v1.0-01 | Schema freeze + versioning | Versioned public schema; migration policy documented; breaking-change rules | Schema version stamped in DB + exports; policy in docs |
| v1.0-02 | Adapter SDK | Go interface + conformance test kit so third parties can build adapters; register externally | A toy third-party adapter passes the kit |
| v1.0-03 | Documentation set | Schema reference, adapter guide, CLI reference, privacy/redaction doc, README polish | Docs cover everything a new user/contributor needs |
| v1.0-04 | Distribution | Release binaries via CI (cross-compile), tags, changelog; install instructions | A user installs v1.0 from a release artifact on macOS/Linux |
| v1.0-05 | Hardening pass | Dogfooding-driven bugfixes; performance on large session stores; fixture suite green on both adapters | No known critical bugs; large-store queries acceptable |

---

# Post-1.0 — evidence-gated backlog (themes, not commitments)

Nothing here is scheduled. Each theme is revisited only when usage evidence justifies it.

| Theme | Note |
|---|---|
| Live watch mode (`agentlens watch`) | Candidate to pull into v0.3+ if tailing files proves valuable in dogfooding |
| OTLP receiver | Makes any OTel-emitting agent work for free; revisit after OTel GenAI semconv stabilizes |
| Codex adapter | Third adapter; only after the SDK exists and Codex exposes usable local data |
| Routing advisor (policy generator) | Config suggestions consumed by the agent's own router; never a proxy |
| Experiment conventions | Templates + labeling protocols; software only if volume demands |
| CI / PR reporting | Reporting-only; never a gate |
| Content snapshot/export mode | Explicit opt-in for reproducibility (D28 allows it) |
| Smarter redaction | Entropy scanning, LLM-assisted PII detection (beyond v0.1 regex set) |
| Team server | Indefinitely deferred — different product, privacy hairball |

---

## Working agreement for issues

1. **No issue starts without its own mini-plan** (scope, schema impact, fixtures, test approach) — produced at kickoff by the Implementer/Architect pairing, per our discovery-phase style.
2. **Sizing check:** if an issue feels bigger than ~3 focused days, split it; if it's under ~half a day, merge it with a sibling.
3. **Fixtures are mandatory** for anything touching adapters, normalization, or metrics.
4. **Schema changes require an ADR** (Architect agent) — no exceptions.
5. **Provenance discipline applies to code too:** no metric ships without its provenance label; no inferred view ships without `[inferred]` marking.
6. **Release gates are questions, not dates:** a version ships when its question is honestly answerable on real sessions.

## Issue-tracking setup (GitHub project)

- **Board:** <https://github.com/users/AhmedEnnaime/projects/7> (repo: `AhmedEnnaime/AgentLens`)
- **Status field:** `Planning → Todo → In Progress → In Review → Done` (existing columns confirmed sufficient)
- **Labels:** chosen from the proposed set below (see proposal in the conversation; final list to be applied at bootstrap)
- **Milestones:** one GitHub milestone per release: `v0.1`, `v0.2`, `v0.3`, `v0.4`, `v0.5`, `v1.0`
- **Each issue carries:** the release milestone, labels, and links to its parent-release tracking issue (parent-issue feature enabled on the board)