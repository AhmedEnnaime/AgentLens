# AgentLens Architecture Decision Records

ADRs document the decisions that shape AgentLens's architecture. Each is numbered, immutable once accepted (superseded by a new ADR, never edited), and required for schema changes per AGENTS.md §3.

| # | Title | Status |
|---|---|---|
| [0001](0001-canonical-model-v1.md) | Canonical Model v1 — Hybrid OTel Trace/Span/Event Types with Provenance | Accepted |
| [0002](0002-sqlite-storage-schema.md) | SQLite Storage Schema, Repository Contract, and Migration Policy | Accepted |

## Format

- **Status:** Proposed → Accepted (owner approval required) → Superseded by `NNNN`
- Sections: Context → Decision → (evidence, type inventory, consequences as needed)
- An accepted ADR is never edited in place; changes come through a superseding ADR.