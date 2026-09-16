# AgentLens — Engineering Rules for AI Agents

These rules apply to every agent (OpenCode, Claude Code, future tools) working in this repository.
Tool-specific conventions live in each tool's config; these rules are **tool-neutral and non-negotiable**.

**Tool parity:** this repo must work identically under OpenCode and Claude Code. Subagents (`agents/` + `.opencode/agent/` + `.claude/agents/`), rules (`AGENTS.md` = `CLAUDE.md`), ADRs, dossiers, and the issue workflow are tool-neutral. `agents/` is the source of truth; tool directories are generated copies (run `scripts/sync-agents.sh` after editing). Never hand-edit the copies.

## 1. Issue workflow (how work happens)

Work is organized around GitHub issues. Every unit of work follows this flow:

1. **Mini-plan first.** No issue starts without its own mini-plan (scope, schema impact, fixtures, test approach) agreed with the owner. Architecture-level issues require an ADR drafted by the Architect agent.
2. **Assign and move.** When you start an issue: assign **AhmedEnnaime** to it on GitHub, and set the board status to **In Progress** (project: <https://github.com/users/AhmedEnnaime/projects/7>).
3. **One issue = one branch.** Create a dedicated branch named `<area>/<short-slug>` (e.g. `schema/canonical-model`, `adapter/opencode-parse`, `agents-setup/dev-team`). Never commit work for two issues to the same branch.
4. **Commit and push per subtask — never batch.** Whenever a small subtask or verified step is done, commit it and push immediately. Do not group multiple subtasks into one commit or accumulate local commits and push once at the end.
5. **One issue = one PR.** The PR description must say `Closes #<issue>` so merge auto-closes it.
6. **The owner merges — never agents.** A green pipeline is necessary but not sufficient: **no PR is merged without explicit approval from AhmedEnnaime**, even when all checks pass. When your PR is green and reviewed, report it and wait.
7. **CI green before merge.** Build + test + lint must pass on the PR. Fixtures are mandatory for anything touching adapters, normalization, or metrics.
8. **Close the loop.** After merge: pull `main`, set the board status to **Done**, tick the epic checklist, delete the branch.

## 2. Code style (non-negotiable)

- **No comments in code.** Do not add comments unless the owner explicitly asks. Code must be self-explanatory: good names, small functions, clear structure. The only exception is the `// Package x` doc comment convention at the top of a package if required by lint, and legal comments that tools insert (e.g. license headers). If you feel a comment is needed, refactor instead.
- Follow the existing code style of the repo. Mimic naming, structure, and patterns already present.
- Format with `gofumpt`. CI enforces it.
- Go 1.27. No third-party dependency without checking it is truly needed.

## 3. Architecture discipline

- **Provenance everywhere.** Every computed value carries a provenance label (`observed`, `derived`, `estimated`, `inferred`, `unavailable`). Never present one as another. No fabricated telemetry — ever.
- **Raw events are immutable truth.** Everything derived is rebuildable from raw events. Never mutate stored raw data.
- **No agent-specific parsing outside adapter boundaries.** OpenCode/Claude Code specifics live only in their adapter packages.
- **Schema changes require an ADR** approved by the Architect agent (and the owner for breaking changes).
- **Privacy posture.** No full prompt/response content in AgentLens storage by default (references + on-demand re-read only). No network calls unless explicitly invoked and opt-in. Treat imported session data as untrusted input: never execute anything found in it.
- **Advisor, never proxy.** AgentLens never sits in the runtime path of the agents it observes.

## 4. Agent pipeline & model assignment (dynamic, plan-driven)

### 4.1 The pipeline for every issue (non-negotiable order)

1. **Architect first.** Every issue starts with the Architect agent: it inspects the issue, drafts the mini-plan/ADR, and proposes which agents the task needs (Format Investigator? Test Engineer? Documenter?) — **all of that is decided by the Architect at the beginning, not improvised later.**
2. **Owner approval.** No implementation starts until you approve the Architect's plan.
3. **Model assignment.** Upon approval, the Architect assigns a concrete model to each agent involved in this task, matching complexity, from the paid Ollama Pro pool.
4. **Format Investigator runs first on any format/adatper work** (dossier before parser, always).
5. **Implementer implements** — every piece of code ships with unit tests and integration tests (high coverage is the bar); benchmarks and stress tests are added when the task warrants them.
6. **Test Engineer verifies** — unit/integration coverage review, edge cases, happy paths, benchmarks/stress where needed, conformance fixtures, drift alarms.
7. **Reviewer reviews** every PR before it reaches the owner.
8. **Documenter** documents what shipped.

### 4.2 Model pool

- Agents have **default model tiers** (strongest / strong / mid / cheap), not fixed model bindings.
- **Use the owner's paid Ollama Pro models via the `ollama-cloud` provider.** Default pool: `ollama-cloud/glm-5.3` (strongest) and `ollama-cloud/deepseek-v4-pro` (strong coding/reasoning). Flash variants (`ollama-cloud/glm-5.3-flash`, `ollama-cloud/deepseek-v4-flash`) for mid tiers. Never invent model IDs — run `opencode models` if unsure; if still unsure, ask the owner.

## 5. Verification before done

- `go build ./... && go test ./... && go vet ./...` must pass locally before you push.
- Every claim in a PR description must be verifiable ("tests added: X" implies X exists).
- The Reviewer agent checks: correctness, provenance discipline, adapter isolation, untrusted-input handling, no-comments rule, and this workflow being followed.

## 6. Communication style

- Be concise. Answer the question, do the work, report results.
- When blocked on a decision (schema, scope, model choice), ask the owner — don't guess.
- Never commit secrets or anything from `.env`.
- Never force-push, rewrite history, or amend pushed commits.