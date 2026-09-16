# The AgentLens Development Agent Team

Six agents, defined tool-neutrally in `agents/`, generated into `.opencode/agent/` and `.claude/agents/` by `scripts/sync-agents.sh`. Never hand-edit the copies — edit `agents/` and re-run the script.

## The team

| Agent | Mandate | Default model (Ollama Pro, `ollama-cloud` provider) |
|---|---|---|
| **Architect** | Leads every issue: mini-plan/ADR, decides which agents run, assigns per-task models after owner approval | `ollama-cloud/glm-5.3` |
| **Format Investigator** | Reverse-engineers session formats; format dossiers + anonymized golden fixtures; runs before any adapter work | `ollama-cloud/deepseek-v4-pro` |
| **Implementer** | Feature/adapter code from approved plans; small verified steps; unit + integration tests with everything; **no comments in code** | `ollama-cloud/deepseek-v4-pro` |
| **Test Engineer** | Verifies implemented work: unit/integration coverage, edge cases, happy paths, benchmarks/stress when warranted; conformance fixtures, drift alarms | `ollama-cloud/deepseek-v4-pro` |
| **Reviewer** | Defect-focused review of every PR; invariant checklist (provenance, adapter isolation, untrusted input, no-comments rule, workflow) | `ollama-cloud/glm-5.3` |
| **Documenter** | Docs from code + dossiers only; runs last, after review; never documents what doesn't exist | `ollama-cloud/deepseek-v4-pro` |

## The pipeline (every issue, non-negotiable order)

```text
Issue arrives
    ↓
Architect: mini-plan/ADR + decides which agents this task needs + proposes models
    ↓
Owner approves the plan
    ↓
Architect assigns concrete models to each agent for this task
    ↓
[Format work?] Format Investigator (dossier before parser, always)
    ↓
Implementer (unit + integration tests with every piece of code; benchmarks/stress when warranted)
    ↓
Test Engineer (coverage review, edge cases, happy paths, conformance, drift)
    ↓
Reviewer (defect-focused, invariant checklist)
    ↓
Owner merges (never agents — explicit approval required)
    ↓
Documenter (documents what shipped)
```

## Model policy (dynamic, plan-driven)

- The defaults above are **tiers, not bindings**: strongest (`glm-5.3`), strong coding/reasoning (`deepseek-v4-pro`), flash variants for mid tiers — all from the owner's paid Ollama Pro subscription via the **`ollama-cloud` provider prefix**.
- The Architect assigns a concrete model to each agent **per task**, matching complexity, after the owner approves the plan. Assignments change task to task.
- Never invent model IDs — run `opencode models` if unsure; ask the owner if still unsure.
- AgentLens itself will one day measure which assignments are optimal (dogfooding closes the loop).

## Workflow recipes

```text
New adapter:   Investigator (dossier) → Architect (contract review) → Implementer → Test Engineer (fixtures) → Reviewer
Schema change: Architect (ADR) → Implementer → Reviewer → Documenter
Feature:       mini-plan → Implementer → Reviewer → Test Engineer
```

## Issue workflow (all agents)

1. Mini-plan agreed with the owner before work starts.
2. Assign AhmedEnnaime on GitHub; board status **In Progress** (project: https://github.com/users/AhmedEnnaime/projects/7).
3. Branch `<area>/<short-slug>` — one issue, one branch, one PR.
4. **Commit and push per subtask — never batch.** Each small, verified step gets its own commit, pushed immediately.
5. PR says `Closes #<issue>`; CI must be green.
6. **The owner merges — never agents.** Report the green PR and wait for approval.
7. After merge: pull `main`, board status **Done**, tick the epic checklist, delete the branch.

## Tool parity (OpenCode and Claude Code)

Everything works identically in both tools:

| Artifact | Source of truth | OpenCode | Claude Code |
|---|---|---|---|
| Engineering rules | `AGENTS.md` | loaded via `instructions` | copy as `CLAUDE.md` |
| Agent definitions | `agents/*.md` | `.opencode/agent/` | `.claude/agents/` |
| Permissions | — | `opencode.json` (auto mode + deny-list) | `.claude/settings.json` (allow-list + deny-list) |
| ADRs / dossiers | `docs/` | same | same |
| Issue workflow | GitHub | same | same |

After editing any agent or the rules: run `scripts/sync-agents.sh`, then restart the tool (configs load at startup).