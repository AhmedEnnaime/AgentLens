# AgentLens — Architecture Finalization Decisions & Pre-Implementation Review

**Status:** Proposed for final architecture review  
**Purpose:** Feed the latest architectural decisions and remaining clarifications back into OpenCode before implementation begins.

This document is a follow-up to `AgentLens_Brainstorming_Report.md`.

## 1. Architecture Direction

AgentLens is a standalone, local-first, agent-agnostic **flight recorder for AI-assisted software engineering**.

Its responsibility is:

```text
OBSERVE
   ↓
RECORD
   ↓
RECONSTRUCT
   ↓
ANALYZE
   ↓
EXPLAIN
   ↓
RECOMMEND
```

It is **not** a coding agent, model proxy, or autonomous workflow controller.

Founding principle:

> **The observer measures reality; the LLM explains reality.**

Every value must be classified as:

```text
observed
derived
estimated
inferred
unavailable
```

## 2. Recommended Owner Decisions

### Q1 — Go or Rust?

**Decision: Go**

Reason:

- Early versions will have significant schema churn.
- Adapter formats will change frequently.
- The workload is primarily I/O and data processing.
- Go provides fast iteration and simple cross-platform distribution.
- The MVP does not need Rust's performance advantages.
- The language decision is reversible; the data model is more important.

Rust remains a valid future option if a concrete technical reason appears.

### Q2 — OTel hybrid or fully bespoke schema?

**Decision: Hybrid**

Use OpenTelemetry GenAI semantic conventions as the base vocabulary for concepts that naturally map to model/LLM telemetry.

Use AgentLens-specific extensions for coding-agent semantics that are not adequately represented by the base conventions.

```text
OpenTelemetry / GenAI vocabulary
        +
AgentLens coding-agent extensions
        ↓
AgentLens normalized representation
```

The schema remains provider-neutral.

### Q3 — Microscope or calculator?

**Decision: Microscope**

The flagship product is **session inspection / forensics**.

The calculator is the hook:

```text
tokens
cost
time
```

The microscope is the product:

```text
why did the agent do this?
what happened?
which model did what?
which agent consumed the resources?
where did retries happen?
what context was repeated?
what was verified?
```

Positioning:

> **AgentLens is a flight recorder and microscope for AI coding agents.**

### Q4 — Founding constraints / runtime interposition

**Decision: Yes, with refined wording**

AgentLens core must NOT require runtime model-call proxying/interposition.

It must:

- never execute commands from imported session data
- never silently modify the user's workflow
- never fabricate missing telemetry
- never gate CI based on token counts
- never require a hosted service
- never make runtime interception a prerequisite
- keep the deterministic core fully offline; any network-dependent feature (e.g. `explain` via an Ollama endpoint, pricing updates) is explicitly invoked and opt-in

Preferred wording:

> **AgentLens observes agents; it does not need to sit in the runtime path.**

### Q5 — Stable upstream OpenCode export format

**Decision: Pursue, but do not block the MVP**

First consume official OpenCode session/export/event surfaces.

In parallel, investigate proposing or contributing a stable, versioned export format upstream.

### Q6 — Outcome-fidelity tier

**Decision: Tier 1 initially + explicit human labeling**

Early versions may safely make claims about:

- observed token usage
- observed timing
- observed retries
- observed tool activity
- observed in-session tests
- observed final state

Use:

```bash
agentlens label <session> good
agentlens label <session> bad
```

Do not make strong model-quality or equivalence claims until sufficient evidence exists.

### Q7 — Second adapter

**Decision: Claude Code**

OpenCode is the first deep adapter.

Claude Code becomes the second adapter and therefore the first schema-validation milestone.

### Q8 — Name and CLI

**Decision: AgentLens**

Examples:

```bash
agentlens last
agentlens analyze <session>
agentlens inspect <session>
agentlens compare <a> <b>
agentlens label <session> good
agentlens share <session>
agentlens explain <session>
agentlens ui
```

### Q9 — License

**Decision: Apache-2.0**

Reason:

- open-source core
- public adapter ecosystem
- public schema
- SDK potential
- explicit patent grant

### Q10 — LLM analyst

**Decision: NoOp by default; Ollama as the analyst provider**

The deterministic engine must work without an LLM.

Owner's runtime context (confirmed): Ollama is installed and is the current AI runtime; dogfooding uses **Ollama Cloud models** (`glm-5.3:cloud`, `deepseek-v4-pro:cloud`) rather than local inference. `explain` is exercised with Ollama during development.

Use:

```text
default:
NoOpAnalyst
```

When semantic explanation is requested:

```bash
agentlens explain <session>
```

Allow an explicit provider/model:

```bash
agentlens explain <session> --model ollama/glm-5.3:cloud
```

Every semantic analysis result records:

- provider
- model
- prompt/instruction version
- analysis version
- source session
- provenance

### Q11 — Frontend

**Decision: React + Vite + ECharts**

The UI remains v0.5.

It is a read-only view over the same SQLite-derived data used by the CLI.

### Q12 — UI visual identity

**Decision: Dark profiler / developer-forensics style**

Reference mental models:

- browser DevTools
- profiler timelines
- flamegraphs
- debugging tools

Avoid marketing-dashboard aesthetics.

Provenance states should have consistent visual treatment:

```text
observed
inferred
estimated
unavailable
```

## 3. Additional Decisions

### Q13 — Canonical "session" concept

**Decision: Use a canonical Trace as the structural root.**

Different agents have different native concepts:

```text
OpenCode session
Claude Code session
Codex thread
```

Do not assume these are semantically identical.

Normalize them into:

```text
Trace
  ├── spans
  ├── events
  └── attributes
```

An agent-native session is source metadata.

An inferred task is a derived view.

```text
Agent-native session
       ↓
Raw events
       ↓
Canonical trace/spans
       ↓
Derived task view
```

### Q14 — Full prompts/responses by default?

**Decision: Metadata-first; content capture is opt-in.**

Default data should favor:

- model identity
- tokens
- timings
- agent identity
- tool identity
- file paths where appropriate
- event relationships
- hashes/references
- provenance

Full prompts, responses, command output, and diffs should be captured only according to an explicit privacy mode.

Suggested modes:

```text
metadata-only
content-local
redacted-export
```

No cloud transfer happens implicitly.

Content-local refinement (owner-confirmed): AgentLens does **not** store full prompts/responses in its own SQLite by default. It stores hashes/references and **re-reads the original agent session files on demand**. This keeps the database lean and avoids persistent duplication of sensitive content. An explicit future snapshot/export mode may be added when reproducibility requires it.

### Q15 — When should model recommendations appear?

**Decision: Recommendations may appear early, but confidence must be explicit.**

Early recommendations may be heuristic and low-confidence.

Later recommendations can become evidence-backed.

Example:

```text
Recommendation:
Use GLM-5.3 for this task.

Confidence:
LOW

Evidence:
4 comparable historical tasks.
```

## 4. Updated Architecture

```text
                     AgentLens
                         │
              ┌──────────┴──────────┐
              │                     │
             CLI                 Optional UI
              │                     │
              └──────────┬──────────┘
                         │
                   Analysis Core
                         │
                  Raw Event Store
                         │
                Canonical Trace Model
                         │
            ┌────────────┼────────────┐
            │            │            │
         Metrics      Derived Views  Provenance
            │            │            │
            └────────────┼────────────┘
                         │
                 Optional Analyst
                         │
                  Local-first LLM
                         │
                  Recommendations
```

Storage hierarchy:

```text
Raw events
    ↓
Normalized spans
    ↓
Derived views
    ↓
Inference cache
```

All derived information must be rebuildable from raw events.

## 5. What "Hybrid OTel" Means

AgentLens should not invent every telemetry field from scratch.

OpenTelemetry already provides standard concepts for telemetry around model calls and related AI operations.

AgentLens should reuse standard vocabulary where it fits:

```text
model
provider
operation
input tokens
output tokens
latency
trace/span relationships
tool-related GenAI attributes
```

But coding agents do more than model calls.

AgentLens needs coding-agent-specific concepts such as:

```text
file_edit
file_read
test_run
git_change
human_intervention
agent_execution
skill_invocation
session_compaction
workflow_outcome
task_inference
provenance
session_completeness
```

Therefore:

```text
                 AgentLens
                     │
             ┌───────┴────────┐
             │                │
      OTel / GenAI        AgentLens
       vocabulary          extensions
             │                │
             └───────┬────────┘
                     │
             Canonical Trace
```

Hybrid does **not** mean AgentLens must become an OpenTelemetry Collector or copy the OTel specification wholesale.

OTel is the interoperability vocabulary/base.

AgentLens adds semantics specific to AI-assisted software development.

## 6. Raw Data vs Normalized Data

Store raw agent-native data first.

```text
Raw session data
       ↓
Normalizer
       ↓
Canonical trace/spans
       ↓
Derived views
```

Do not throw away unknown fields during normalization.

Adapters must preserve unknown source fields so newer adapter versions can recover them later.

## 7. Development Agent Team

The six-agent team should be kept.

### 7.1 Architect

Owns:

- canonical schema
- adapter contract
- provenance rules
- ADRs
- cross-cutting architecture

Model tier: **strongest available**

### 7.2 Format Investigator

Owns:

- reverse-engineering agent session formats
- format dossiers
- field mapping
- fixture creation
- format drift analysis

Before implementing a new adapter:

```text
Format Investigator
    ↓
format dossier
    ↓
Architect review
    ↓
Implementer
```

Model tier: **strong reasoning**

### 7.3 Implementer

Owns:

- feature implementation
- adapter implementation
- tests
- small verified changes

Rules:

- no provider-specific parsing outside adapter boundaries
- never fabricate telemetry
- preserve raw fields
- test against real fixtures

Model tier: **strong mid-tier coding model**

### 7.4 Reviewer

Owns:

- defect detection
- provenance correctness
- security
- adapter isolation
- untrusted-input handling
- schema discipline

Use an independently configured model where practical.

Do not encode "different model family is always better" as a hard-coded truth; measure it.

Model tier: **strong**

### 7.5 Test Engineer

Owns:

- golden fixtures
- normalizer tests
- adapter conformance
- schema drift tests
- regression suite

Model tier: **mid**

### 7.6 Documenter

Owns:

- schema reference
- adapter guide
- ADR cleanup
- format dossiers
- README
- user documentation

Model tier: **cheap / fast**

## 8. Agent Team Workflows

### New adapter

```text
Format Investigator
        ↓
Architect
        ↓
Implementer
        ↓
Test Engineer
        ↓
Reviewer
        ↓
Documenter
```

### Schema change

```text
Architect
    ↓
Implementer
    ↓
Reviewer
    ↓
Documenter
```

### Normal feature

```text
Plan
  ↓
Implementer
  ↓
Reviewer
  ↓
Test Engineer
```

Do not add more permanent roles until a recurring responsibility cannot fit into these boundaries.

## 9. Important Change to the Six-Agent Proposal

Do not bind each role permanently to one model.

Use:

```text
role → default model tier
```

For example:

```yaml
architect:
  tier: strongest

format-investigator:
  tier: strong-reasoning

implementer:
  tier: strong-coding

reviewer:
  tier: strong

test-engineer:
  tier: mid

documenter:
  tier: cheap
```

Then let AgentLens eventually measure whether these assignments are actually optimal.

The AgentLens development team becomes its first AI-workflow experiment.

## 10. MVP Boundary

MVP should answer:

> **What happened during this session, and where did the resources go?**

Required:

```text
1. OpenCode adapter
2. Raw session ingestion
3. Raw immutable storage
4. Canonical trace/span model
5. SQLite
6. Per-session summary
7. Per-turn attribution
8. Per-agent attribution
9. Per-model attribution
10. Per-tool attribution
11. Token / latency / cost metrics
12. Provenance
13. Completeness score
14. Redacted share/export
```

Task-level inference remains a later derived view.

## 11. Do Not Build in v0.1

Do not build:

```text
automatic model routing
runtime proxy
team server
authentication
CI gates
experiment dashboard
hosted SaaS
large analytics dashboard
complex task classifier
automatic workflow modification
```

## 12. Core Product Rule

AgentLens should prefer:

```text
"What we know"
```

over:

```text
"What we think happened"
```

Example:

```text
Observed:
Implementer used GLM-5.3 for 4 model calls.

Inferred:
These calls likely belong to the retry-logic task.

Unavailable:
Provider did not report cached-token usage.

Estimated:
Cost calculated from published model pricing.
```

## 13. Final Pre-Implementation Checklist

Before coding starts, confirm:

- [x] Go is selected
- [x] Apache-2.0 is selected
- [x] AgentLens is selected
- [x] hybrid OTel direction is accepted
- [x] canonical root is Trace, not Task
- [x] raw-event storage is immutable/reprocessable
- [x] OpenCode is the first deep adapter
- [x] Claude Code is the second adapter milestone
- [x] full content is opt-in (references + on-demand re-read; no content copies in AgentLens SQLite by default)
- [x] no runtime proxy is required
- [x] no automated routing in MVP
- [x] six development agents are accepted
- [x] React + Vite + ECharts remains v0.5
- [x] no server/auth in early versions
- [x] deterministic analysis works without an LLM
- [x] Ollama is the first Analyst provider (confirmed: Ollama installed; dogfooding via Ollama Cloud models `glm-5.3:cloud`, `deepseek-v4-pro:cloud`)
- [x] provenance and completeness are first-class
- [x] fixture-driven adapter testing is mandatory
- [x] roadmap v0.1 → v0.5 sequence confirmed as-is (owner)
- [x] offline wording fixed: deterministic core works fully offline; network features are explicit and opt-in

All items confirmed (2026-09-14). The v0.1 implementation plan can now be produced.
