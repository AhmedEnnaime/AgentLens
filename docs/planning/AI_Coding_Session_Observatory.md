# AI Coding Session Observatory
## Product Vision, Architecture, Roadmap, and Ecosystem

**Status:** Concept / Proposed  
**Document purpose:** Define a standalone, agent-agnostic observability and optimization platform for AI coding sessions.

---

# 1. Executive Summary

AI coding agents such as OpenCode, Claude Code, Codex, and other agentic development tools perform increasingly complex workflows:

```text
User request
    ↓
Agent
    ↓
Model
    ↓
Subagents
    ↓
Skills
    ↓
Tools
    ↓
Tests
    ↓
Reviews
    ↓
Fix iterations
```

The difficulty is that developers usually do not have a clear picture of what actually happened during that workflow.

This project aims to provide an **AI Coding Session Observatory** that observes, normalizes, analyzes, and improves AI-assisted software-development sessions.

The system should answer:

> What happened during this AI coding session?

> How much did every part cost in tokens, time, and money?

> Which agents, models, skills, and tools contributed to the result?

> Where was context or token usage wasted?

> Which model would have been more appropriate for each task?

> Did the additional model/agent actually improve the outcome?

> How can future sessions achieve the same or better engineering result at lower cost, lower latency, or lower risk?

The project should eventually evolve from an observability tool into an **AI engineering optimization and model-routing platform**.

---

# 2. Core Product Principle

The most important rule of the system is:

> **The observer measures reality; the LLM explains reality.**

Deterministic telemetry must be used for:

- token counts
- cached token counts
- model identifiers
- timestamps
- durations
- tool calls
- agent identifiers
- session identifiers
- file changes when observable
- API/provider costs when available

LLMs may be used for:

- semantic task classification
- task decomposition inference
- workflow interpretation
- root-cause analysis
- recommendation generation
- model comparison
- optimization suggestions

The LLM must never invent telemetry that was not observed.

---

# 3. Product Vision

The product should evolve through four layers:

```text
OBSERVE
    ↓
ANALYZE
    ↓
OPTIMIZE
    ↓
ROUTE
```

## 3.1 Observe

Capture what happened.

## 3.2 Analyze

Explain how resources and workflow were used.

## 3.3 Optimize

Suggest changes that preserve or improve outcome quality.

## 3.4 Route

Eventually help choose:

- workflow
- agent
- model
- number of reviewers
- level of verification
- context strategy

The system should not start by automatically changing user workflows. Automation should be introduced only after enough evidence has been collected.

---

# 4. Primary Goals

## Goal 1 — Multi-agent observability

Support coding environments including:

- OpenCode
- Claude Code
- Codex
- future coding agents

without coupling the core system to one vendor.

## Goal 2 — Token and cost visibility

Measure:

- input tokens
- cached input tokens
- output tokens
- reasoning/thinking tokens when exposed
- estimated cost
- provider cost when directly available

## Goal 3 — Time visibility

Measure:

- total session duration
- time per user turn
- time per model call
- time per agent
- time per tool call
- waiting time
- retry time
- parallel execution

## Goal 4 — Workflow visibility

Understand:

- prompts
- tasks
- subtasks
- agents
- subagents
- skills
- model calls
- tools
- tests
- review rounds
- fix rounds
- retries
- session compaction
- human intervention

## Goal 5 — Outcome-aware optimization

Never optimize only for token reduction.

The objective is:

```text
same or better engineering outcome
+
less waste
```

Potential dimensions:

- correctness
- test success
- defect detection
- regression risk
- human intervention
- cost
- latency
- token efficiency

## Goal 6 — Learn from historical sessions

Use historical data to understand:

- which models work best for which task types
- which agents add value
- which skills consume significant context
- where repeated context occurs
- which workflows create excessive retries
- which review strategies detect real defects

---

# 5. Non-Goals

The first versions should NOT attempt to:

- automatically modify project configuration
- automatically change model routing
- replace coding agents
- act as a generic AI assistant
- guarantee semantic task decomposition
- fabricate missing telemetry
- optimize solely for lowest token usage
- require every supported agent to expose identical telemetry

---

# 6. Product Form

The recommended primary product is:

```text
Standalone Go application
    +
CLI
    +
Normalized telemetry/analysis engine
    +
Adapter system
    +
SQLite
    +
Optional local web dashboard
```

---

# 7. Why a Standalone Tool?

The problem is cross-project and cross-agent.

It concerns:

- developers
- AI coding workflows
- model routing
- cost management
- agent orchestration
- engineering productivity

These concepts should not be embedded in one application's source code.

A separate tool also makes it possible to eventually distribute it to other developers.

---

# 8. Proposed Architecture

```text
                         ┌──────────────────────────┐
                         │       CLI / Web UI       │
                         └────────────┬─────────────┘
                                      │
                         ┌────────────▼─────────────┐
                         │     Analysis Engine      │
                         └────────────┬─────────────┘
                                      │
                         ┌────────────▼─────────────┐
                         │ Normalized Event Model   │
                         └────────────┬─────────────┘
                                      │
                 ┌────────────────────┼────────────────────┐
                 │                    │                    │
          OpenCode Adapter      Claude Adapter       Codex Adapter
                 │                    │                    │
                 └────────────────────┼────────────────────┘
                                      │
                              Agent-native data
                                      │
                         ┌────────────▼─────────────┐
                         │       SQLite Store       │
                         └──────────────────────────┘
```

Optional:

```text
                    ┌───────────────────────┐
                    │ LLM Analysis Service  │
                    └───────────┬───────────┘
                                │
                       structured metrics
                                │
                       semantic interpretation
                                │
                         recommendations
```

---

# 9. Adapter Architecture

Adapters are the key to agent compatibility.

Do NOT scatter agent-specific parsing throughout the system.

Instead define a common adapter contract.

Conceptually:

```go
type AgentAdapter interface {
    Name() string
    DiscoverSessions(...) (...)
    ReadSession(...) (...)
    StreamSession(...) (...)
    NormalizeEvent(...) (...)
    Capabilities() (...)
}
```

Potential adapters:

```text
OpenCodeAdapter
ClaudeCodeAdapter
CodexAdapter
FutureAgentAdapter
```

The core system must consume only normalized events.

---

# 10. Capability-Based Compatibility

Not every coding agent exposes the same information.

Therefore each adapter should declare capabilities.

Example:

```yaml
agent: opencode

capabilities:
  session_metadata: true
  model_usage: true
  token_usage: true
  tool_calls: true
  agent_identity: true
  skill_identity: true
  file_changes: true
  live_events: true
```

Another agent may have:

```text
token_usage: false
skill_identity: false
live_events: true
```

The analyzer must explicitly distinguish:

```text
observed
inferred
estimated
unavailable
```

Never silently convert missing telemetry into fake precision.

---

# 11. Canonical Event Model

The normalized event model is the most important technical component.

A session should be represented as a hierarchy:

```text
Session
  └── Task
       ├── Agent execution
       │     ├── Model invocation
       │     ├── Skill invocation
       │     ├── Tool calls
       │     └── Result
       │
       ├── Agent execution
       │     └── ...
       │
       └── Outcome
```

Each event should support:

```json
{
  "session_id": "ses_xxx",
  "event_id": "evt_xxx",
  "parent_event_id": "evt_parent",
  "timestamp": "2026-09-12T18:00:00Z",
  "type": "model_call",
  "agent": "architect",
  "model": {
    "provider": "ollama",
    "name": "glm-5.3:cloud"
  },
  "tokens": {
    "input": 12000,
    "cached_input": 7000,
    "output": 3200
  },
  "duration_ms": 42000,
  "telemetry_confidence": "exact"
}
```

Other event types may include:

```text
session_started
session_ended
user_prompt
task_created
task_decomposed
agent_started
agent_completed
model_call
tool_call
skill_invoked
file_read
file_edit
command_started
command_completed
test_started
test_completed
review_started
review_completed
retry
compaction
human_intervention
task_completed
task_failed
```

---

# 12. Observed vs Inferred Data

This distinction is critical.

## Observed

Directly reported by the provider/agent:

```text
model = GLM-5.3
input_tokens = 12000
duration = 12.2 seconds
```

## Inferred

Derived from event relationships:

```text
This model call belongs to the architect task.
```

## Estimated

Calculated from incomplete information:

```text
Estimated cost = tokens × published model price
```

Every metric should retain its provenance.

Example:

```yaml
value: 0.42
source: calculated
confidence: estimated
```

---

# 13. Metrics

## 13.1 Token Metrics

Measure:

- input tokens
- cached input
- output tokens
- reasoning tokens when available
- total tokens
- tokens per task
- tokens per agent
- tokens per model
- tokens per skill
- repeated-context tokens

## 13.2 Timing Metrics

Measure:

- total duration
- time per model call
- time per agent
- time per task
- tool execution time
- waiting time
- retry time
- parallel execution time

## 13.3 Cost Metrics

Measure:

- provider-reported cost
- estimated cost
- cost by model
- cost by agent
- cost by task
- cost by workflow
- cost per successful task

Never show an estimated number as though it were provider-reported billing.

## 13.4 Workflow Metrics

Measure:

- number of agents
- number of subagents
- number of model calls
- number of tool calls
- number of skill invocations
- number of retries
- number of fix rounds
- number of compactions
- number of human interventions

## 13.5 Outcome Metrics

Where observable:

- tests passed
- tests failed
- review findings
- real defects found
- rework
- final status
- human corrections
- task completion

---

# 14. Session Tree / Replay

A major product feature should be a visual session tree.

Example:

```text
03:00 User Request
   │
   ├── Explorer
   │     └── Model A
   │
   ├── Architect
   │     └── Model B
   │
   └── Implementer
         ├── Model C
         ├── Read
         ├── Edit
         ├── Bash
         └── Tests
               │
               └── Reviewer
                     └── Model D
```

Users should be able to inspect each node.

For example:

```text
ARCHITECT

Model: GLM-5.3
Duration: 38s

Input: 82,401
Cached: 61,201
Output: 9,213

Skills:
- architecture
- brainstorming

Files inspected: 17

Estimated cost: $0.42

Outcome:
Plan accepted
```

---

# 15. First CLI

The first release should be CLI-first.

Possible commands:

```bash
aiobs sessions
aiobs analyze <session>
aiobs analyze --agent opencode <session>
aiobs inspect <session>
aiobs report <session>
aiobs compare <session1> <session2>
aiobs recommendations <session>
aiobs import <file>
aiobs export <session>
aiobs watch
```

---

# 16. Dashboard

After the CLI works, add an optional local dashboard.

Potential views:

- Session Overview
- Session Timeline
- Agent Breakdown
- Model Breakdown
- Context Analysis
- Recommendation Center
- Historical Trends
- Experiment Results

---

# 17. Optimization Engine

The tool should eventually detect "AI workflow smells."

## Context Waste

Repeated large context.

## Model Mismatch

Expensive model performing a simple task.

## Underpowered Model

Repeated retries or escalating failures.

## Agent Thrashing

Repeated switching between agents without progress.

## Excessive Retries

Same task repeatedly attempted.

## Tool Waste

Repeated file searches or reads.

## Skill Waste

Skill loaded but not meaningfully used.

## Review Inefficiency

Expensive review repeatedly finds nothing on low-risk tasks.

## Context Overflow

Frequent compaction.

## Unnecessary Parallelism

Multiple agents performing overlapping work.

---

# 18. Recommendations Must Be Outcome-Aware

Do NOT optimize for:

```text
minimum tokens
```

Optimize for:

```text
same or better engineering result
+
lower cost
+
lower latency
+
reasonable risk
```

The analyzer should compare recommendations against outcome evidence.

Bad recommendation:

> Use a cheaper model because it uses 70% fewer tokens.

Better recommendation:

> Comparable low-risk documentation tasks in this repository have achieved the same verification outcome with GLM-5.3-Flash at substantially lower cost. Consider routing documentation to Flash.

---

# 19. Model Evaluation

The system should eventually become a model-evaluation laboratory.

Track:

```text
task type
model
agent
skills
cost
latency
tests
review findings
rework
human intervention
final result
```

The goal is to answer:

> Which model works best for this task category in this environment?

Not:

> Which model has the highest benchmark score?

---

# 20. Model Routing

Long-term, the project can provide model-routing recommendations.

Pipeline:

```text
Task
  ↓
Task Classification
  ↓
Complexity / Risk / Domain
  ↓
Historical Performance
  ↓
Candidate Models
  ↓
Cost / Quality Tradeoff
  ↓
Recommendation
```

Task profile could include:

```yaml
complexity: high
risk: high
domain:
  - payments
  - concurrency
required_capabilities:
  - coding
  - security-reasoning
context_size: large
```

Potential recommendation:

```yaml
recommended_model: glm-5.3
reason:
  - high-risk concurrency change
  - large context
  - historical success on similar tasks
fallback: qwen
```

---

# 21. Human-in-the-Loop Model Routing

The initial implementation should NOT automatically choose models.

Recommended progression:

## Stage 1

Human chooses.

The system records the choice and outcome.

## Stage 2

System recommends.

Human approves or changes the recommendation.

## Stage 3

System auto-routes low-risk tasks.

Human approval remains required for high-risk tasks.

## Stage 4

Fully automated routing for selected low-risk workflows.

---

# 22. Orchestrator Integration

Eventually the observability platform can expose an API that orchestrators consume.

```text
Orchestrator
     ↓
AI Session Observatory
     ↓
historical data
     ↓
model recommendation
     ↓
Orchestrator
```

The recommendation engine can answer:

```text
"What model should I use for this task?"
```

based on actual historical evidence.

---

# 23. Experimental Model Routing

Support controlled experiments.

```text
Experiment:
Review Model Comparison

Implementation:
GLM-5.3

Group A:
Reviewer = GLM-5.3-Flash

Group B:
Reviewer = Model B

Measure:
- real bugs caught
- false positives
- cost
- latency
- human adjudication
```

---

# 24. Session Comparison

Support:

```bash
aiobs compare session-A session-B
```

Example dimensions:

```text
cost
duration
tokens
retries
review findings
tests
human intervention
```

Then explain whether one workflow achieved a similar or better verified outcome with fewer resources.

---

# 25. Cross-Session Learning

Aggregate historical data to answer:

- What tasks are most expensive?
- Which models are overused?
- Which agents create the most rework?
- Which skills consume the most context?
- Which workflows are slow?
- Which models detect the most real defects?
- Where does human intervention happen most frequently?

---

# 26. Privacy and Security

This is essential if the tool is distributed to other developers.

Coding sessions may contain:

- source code
- secrets
- API keys
- credentials
- proprietary prompts
- architecture
- customer data
- private repository paths

The default architecture should be **local-first**.

No data should leave the machine by default.

Users should explicitly opt in to:

- cloud analysis
- telemetry upload
- model providers
- team dashboards

Add secret redaction for:

```text
API keys
tokens
passwords
private keys
.env values
authorization headers
```

Potential commands:

```bash
aiobs scrub session.json
aiobs export --redacted session.json
```

The system should show what will be sent to an external model.

---

# 27. Privacy Modes

Support:

```text
local-only
redacted-cloud
full-cloud
```

### local-only

All telemetry and semantic analysis remain local.

### redacted-cloud

Sanitized structured data may be sent to an LLM.

### full-cloud

Explicitly enabled by the user.

---

# 28. Security of Imported Sessions

Session files must be treated as untrusted input.

Do not execute:

- shell commands
- scripts
- tool calls
- project files

simply because they appear in recorded session data.

Parsing must be data-only.

---

# 29. Storage

Initial storage:

```text
SQLite
```

Potential tables:

```text
sessions
events
tasks
agents
model_calls
tool_calls
skills
token_usage
timings
outcomes
recommendations
experiments
model_profiles
```

Avoid introducing PostgreSQL until there is a demonstrated need for a shared multi-user service.

---

# 30. Import and Export

Users should be able to save and move session data.

Formats:

```text
JSON
JSONL
CSV
```

The canonical export should be self-contained enough for offline analysis.

---

# 31. Open Telemetry Schema Direction

Consider defining a small public, provider-neutral telemetry schema.

Core entities:

```text
session
task
agent
model
skill
tool
token_usage
timing
cost
outcome
provenance
```

This creates the possibility for:

- third-party adapters
- community integrations
- CI integrations
- IDE integrations
- research experiments
- external dashboards

---

# 32. Adapter SDK

After the core model stabilizes, provide an adapter SDK.

A new integration should ideally implement:

```text
discover
parse
normalize
capabilities
```

Potential future adapters:

```text
opencode
claude-code
codex
cursor
custom-internal-agent
```

---

# 33. OpenCode Integration

OpenCode should be the first integration.

Potential levels:

1. import exported sessions
2. query OpenCode sessions
3. OpenCode plugin sends live events
4. live observability via `aiobs watch`

The plugin should be an adapter, not the core product.

---

# 34. Claude Code Integration

Potential levels:

1. read exported/structured session data
2. normalize Claude Code events
3. optional live integration where supported

Claude-specific semantics must remain inside the adapter.

---

# 35. Generic Agent Support

For unknown agents:

```bash
aiobs import --format generic events.jsonl
```

This gives the ecosystem a fallback before a native adapter exists.

---

# 36. LLM Analyst Architecture

Use deterministic analysis first.

```text
Raw Telemetry
      ↓
Deterministic Metrics
      ↓
Structured Analysis
      ↓
LLM Analyst
      ↓
Recommendations
```

Do not ask an LLM to count tokens.

Do not ask an LLM to calculate provider billing when exact pricing is available.

Use an LLM to interpret structured evidence.

---

# 37. Recommendation Confidence

Recommendations should carry confidence and evidence.

Example:

```yaml
recommendation:
  type: model_routing
  confidence: high
  evidence:
    - 17 comparable tasks
    - 14 successful outcomes
    - lower average cost
```

---

# 38. Recommendation Lifecycle

Every recommendation should follow:

```text
Detected
   ↓
Explained
   ↓
Proposed
   ↓
Experimented
   ↓
Measured
   ↓
Accepted / Rejected
   ↓
Historical evidence
```

Rejected recommendations and their reasons should be retained.

---

# 39. Recommendation Types

Potential categories:

```text
MODEL_ROUTING
CONTEXT_OPTIMIZATION
SKILL_OPTIMIZATION
AGENT_OPTIMIZATION
WORKFLOW_OPTIMIZATION
PARALLELISM_OPTIMIZATION
RETRY_OPTIMIZATION
PROMPT_OPTIMIZATION
VERIFICATION_OPTIMIZATION
COST_OPTIMIZATION
LATENCY_OPTIMIZATION
```

---

# 40. Example Recommendations

### Context

> 41% of input tokens were repeated project context across five model calls.

### Model

> GLM-5.3 was used for documentation tasks where Flash produced equivalent verified outcomes historically. Consider routing documentation to Flash.

### Agent

> The explorer consumed substantial context but did not discover files later used. Consider reducing explorer scope for low-complexity tasks.

### Review

> A flagship reviewer has repeatedly found no real defects on low-risk CRUD tasks. Consider a cheaper reviewer for that task class.

### Retry

> The implementer retried the same task multiple times after deterministic test failures. Add an earlier diagnosis step.

---

# 41. Important Anti-Patterns

## Optimizing for tokens alone

A token reduction that produces worse code is not an optimization.

## More agents = better

Additional agents must demonstrate measurable value.

## Same model self-review

Flag same-model implementation and review as potentially lower-independence.

## Hidden external state

Do not silently depend on private plugin installations or undocumented global configuration.

## Automatic self-modification

Do not let recommendations automatically rewrite the analyzer, project workflow, or model routing without an explicit controlled experiment.

---

# 42. Task Classification

Before model routing, the system may eventually build a task profile:

```text
complexity
risk
domain
language
scope
context size
required capabilities
```

This should influence model selection.

---

# 43. Orchestration and Failure Handling

The system should observe and eventually recommend responses to:

```text
tool failure
compilation failure
repeated test failure
conflicting agent recommendations
insufficient context
hallucinated API
model loop
permission denial
context overflow
```

Track:

```text
failure type
agent
model
retry count
fallback model
final result
```

---

# 44. Human Decision Hierarchy

A recommended hierarchy is:

```text
Human
  ↓
Orchestrator
  ↓
Specialized agents
  ↓
Models
  ↓
Tools
```

The lower layer should not silently override higher-level decisions.

---

# 45. Parallelism and Critical Path

Do not simply sum agent time when agents run in parallel.

Track:

```text
wall_clock_duration
aggregate_agent_duration
parallel_overlap
critical_path_duration
```

Example:

```text
Agent A = 60s
Agent B = 60s
Run in parallel

Wall clock ≈ 60s
Aggregate work = 120s
```

This makes orchestration optimization meaningful.

---

# 46. Session Replay

A future UI should allow users to replay the session logically:

```text
timeline
  ↓
user prompt
  ↓
subagent
  ↓
model call
  ↓
tool call
  ↓
result
```

Each node should show exact or clearly-labeled inferred telemetry.

---

# 47. Experiment Framework

Experiments should be first-class.

Each experiment records:

```text
hypothesis
setup
repository
task category
model
workflow
input
result
metrics
analysis
decision
```

Example:

```text
Hypothesis:
Flash can replace the flagship model for test generation.

Measure:
verified test quality
cost
latency
rework
review findings
```

---

# 48. CI / Git Integration

Future options:

```bash
aiobs ci analyze
```

Potential CI output:

```text
AI-assisted change

Models: 2
Agents: 3
Tokens: 42k
Verification: PASS
Review findings: 2
Human interventions: 1
```

Initially this should be reporting only, not a blocking quality gate.

---

# 49. Pull Request Integration

A future PR comment may show:

```text
AI Session Summary

Implementation:
  GLM-5.3

Review:
  Model B

Tokens:
  92k

Estimated cost:
  $0.64

Verification:
  tests: PASS
  lint: PASS

Recommendation:
  repeated architecture context detected
```

---

# 50. Team Version

After the local version is stable, a server mode could aggregate multiple developers.

```text
Developer A ──┐
Developer B ──┤
Developer C ──┼──> Observatory Server
CI ───────────┘
```

Potential features:

- team dashboards
- shared model evaluation
- organization cost analysis
- shared recommendations
- experiment management

This should come later because it introduces authentication, privacy, retention, and multi-tenancy.

---

# 51. Product Distribution

Recommended long-term split:

## Open-source core

```text
aiobs
```

Includes:

- CLI
- event schema
- local analysis
- local storage
- core adapters

## Optional hosted product

Could provide:

- team dashboards
- centralized analytics
- organization-wide model evaluation
- experiment management
- routing intelligence

Hosted functionality should never be required for local observability.

---

# 52. Proposed Repository Structure

```text
aiobs/
├── cmd/
│   └── aiobs/
├── internal/
│   ├── adapters/
│   │   ├── opencode/
│   │   ├── claude/
│   │   └── codex/
│   ├── analysis/
│   ├── metrics/
│   ├── recommendations/
│   ├── routing/
│   ├── storage/
│   ├── privacy/
│   └── experiments/
├── pkg/
│   ├── event/
│   ├── adapter/
│   └── schema/
├── migrations/
├── docs/
│   ├── architecture/
│   ├── schema/
│   ├── adapters/
│   └── experiments/
├── examples/
└── tests/
```

---

# 53. Technical Stack

Initial recommendation:

```text
Language: Go
Storage: SQLite
CLI: Cobra or equivalent
Serialization: JSON / JSONL
HTTP API: optional
Dashboard: local web UI later
LLM analysis: pluggable provider interface
```

Do not introduce a frontend until the data model and CLI are stable.

---

# 54. LLM Provider Abstraction

The analyzer itself should not depend on one model provider.

Conceptually:

```go
type Analyst interface {
    Analyze(ctx context.Context, input AnalysisInput) (AnalysisResult, error)
}
```

Possible implementations:

```text
OllamaAnalyst
OpenAIAnalyst
AnthropicAnalyst
LocalAnalyst
NoOpAnalyst
```

The deterministic metrics engine must work without an LLM.

---

# 55. Offline-First Design

A user should be able to run:

```bash
aiobs analyze session.json
```

without internet access.

Deterministic analysis should still work.

Only semantic analysis and optional pricing refresh should require network access.

---

# 56. Pricing and Model Catalog

Pricing changes over time.

Store pricing with:

```text
provider
model
input_price
cached_input_price
output_price
effective_date
source
```

Prefer provider-reported cost where available.

Otherwise calculate using a versioned pricing table and label the result as estimated.

---

# 57. Security Model

Treat imported sessions as untrusted data.

Do not execute commands, scripts, or tool calls present in a recorded session.

The analyzer must be read-only by default.

---

# 58. MVP

The first release should answer:

> What happened during this OpenCode session and where did the tokens/time/cost go?

MVP:

```text
✓ OpenCode adapter
✓ Session import
✓ Normalized events
✓ SQLite
✓ Token totals
✓ Model breakdown
✓ Agent breakdown
✓ Tool breakdown
✓ Timing breakdown
✓ Estimated cost
✓ Session tree
✓ CLI report
✓ Data provenance
```

Do NOT include automatic model routing in MVP.

---

# 59. V2

V2 should answer:

> How could this session have been more efficient without reducing quality?

Add:

```text
✓ Context waste
✓ Model mismatch
✓ Retry analysis
✓ Agent efficiency
✓ Skill efficiency
✓ Workflow smells
✓ Recommendations
✓ Confidence
✓ Historical comparisons
```

---

# 60. V3

V3 should answer:

> What should the AI workflow use next time?

Add:

```text
✓ Model evaluation
✓ Experiment framework
✓ Historical task similarity
✓ Model recommendations
✓ Workflow recommendations
✓ Human approval
```

---

# 61. V4

V4 may answer:

> Can the system route low-risk tasks automatically?

Add:

```text
✓ automated model routing for selected tasks
✓ risk-aware routing
✓ escalation
✓ fallback models
✓ human approval thresholds
```

Only implement this after empirical validation.

---

# 62. Success Criteria

The project is successful when a developer can answer:

### Session-level

- What happened?
- Which agent did what?
- Which model did what?
- How many tokens were consumed?
- How much time was spent?
- What did it cost?

### Workflow-level

- Which stage consumed the most resources?
- Where did retries happen?
- Which agents were useful?
- Which skills were useful?
- Where was context duplicated?

### Optimization-level

- Could a cheaper model have achieved the same result?
- Did an expensive reviewer actually add value?
- Which workflow is more efficient?
- What should change next time?

### Strategic-level

- Which models are actually best for my repository?
- Which model is best for which task class?
- What orchestration strategy produces the best engineering outcomes?

---

# 63. Core Product Philosophy

The platform should follow these rules:

1. Measure before optimizing.
2. Optimize outcomes, not token counts.
3. Prefer evidence over benchmark assumptions.
4. Keep telemetry deterministic.
5. Mark inference explicitly.
6. Remain local-first.
7. Keep adapters thin.
8. Keep the core provider-neutral.
9. Let humans approve consequential changes.
10. Use experiments before automating routing.
11. Keep historical data so recommendations can improve.
12. Treat the project itself as an AI-engineering laboratory.

---

# 64. Final Vision

The ultimate purpose is not to build another token dashboard.

The vision is:

> **A universal observability and optimization layer for AI-assisted software engineering.**

The system begins by telling developers:

```text
"What happened?"
```

then evolves to:

```text
"Why did it happen?"
```

then:

```text
"How can we improve it?"
```

and eventually:

```text
"What should the agent do next time?"
```

The progression is:

```text
TELEMETRY
    ↓
OBSERVABILITY
    ↓
ANALYSIS
    ↓
EXPERIMENTATION
    ↓
OPTIMIZATION
    ↓
MODEL EVALUATION
    ↓
MODEL ROUTING
    ↓
AI ENGINEERING INTELLIGENCE
```

Start with one agent (OpenCode), one adapter, deterministic telemetry, SQLite, and a CLI. Prove the event model and analysis engine before building a dashboard or automated router.

