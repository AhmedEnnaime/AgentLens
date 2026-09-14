# The AgentLens Development Agent Team

Six agents, defined tool-neutrally in `agents/`, generated into `.opencode/agent/` and `.claude/agents/` by `scripts/sync-agents.sh`. Never hand-edit the copies — edit `agents/` and re-run the script.

## The team

| Agent | Mandate | Default model (Ollama Pro) |
|---|---|---|
| **Architect** | Schema, ADRs, adapter contract, provenance rules; assigns per-task models after plan approval | `ollama/glm-5.3:cloud` |
| **Format Investigator** | Reverse-engineers session formats; format dossiers + anonymized golden fixtures | `ollama/deepseek-v4-pro:cloud` |
| **Implementer** | Feature/adapter code from agreed plans; small verified steps; **no comments in code** | `ollama/deepseek-v4-pro:cloud` |
| **Reviewer** | Defect-focused review; invariant checklist (provenance, adapter isolation, untrusted input, no-comments rule, workflow) | `ollama/glm-5.3:cloud` |
| **Test Engineer** | Conformance fixtures, golden regression tests, drift alarms | `ollama/deepseek-v4-pro:cloud` |
| **Documenter** | Docs from code + dossiers only; never documents what doesn't exist | `ollama/deepseek-v4-pro:cloud` |

## Model policy (dynamic, plan-driven)

- The defaults above are **tiers, not bindings**: strongest (`glm-5.3:cloud`) and strong coding/reasoning (`deepseek-v4-pro:cloud`) from the owner's paid Ollama Pro subscription.
- When the Architect's plan is approved by the owner, the Architect assigns a concrete model to each agent involved in that task, matching task complexity. Assignments are per-task and changeable.
- Never invent model IDs; when unsure, ask the owner.
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