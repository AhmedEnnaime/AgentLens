# AgentLens

A local-first, open-source **flight recorder for AI coding agents**.

AgentLens observes, reconstructs, and explains what your coding agent (OpenCode first, Claude Code next) actually did during a session — which agents and models ran, where the tokens, time, and cost went, and whether the work verified.

> The observer measures reality; the LLM explains reality.

**Status:** early development (v0.1 — flight recorder for OpenCode). Not yet usable.

## Principles

- **Local-first** — all data stays on your machine; the deterministic core works fully offline; any network feature is explicit and opt-in.
- **Provenance everywhere** — every value is labeled `observed`, `derived`, `estimated`, `inferred`, or `unavailable`. Nothing is fabricated.
- **Advisor, never proxy** — AgentLens observes agents; it never sits in the runtime path and never executes session content.
- **Outcome-aware, not token-obsessed** — efficiency claims are grounded in verified results, not raw token counts.

## Development

- Language: **Go** (1.27) · Storage: **SQLite** · CLI-first, UI at v0.5
- License: **Apache-2.0**
- Releases, issues, and roadmap: see the [GitHub project board](https://github.com/users/AhmedEnnaime/projects/7) and [`docs/planning/`](docs/planning/) in this repo.