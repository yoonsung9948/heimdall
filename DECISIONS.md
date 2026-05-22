# Decisions

Format: one entry per decision. Add to the top (newest first).

---

## [date] [component] — [short title]

**Decision:** what you decided
**Why:** why you made this choice
**Tradeoff:** what you gave up
**Revisit:** when/why to reconsider (or "never")

---

## 2026-05-18 types — PromptArguments removed from v0.1

**Decision:** Removed `PromptArguments` from `PromptDefinition`. Not included in v0.1.
**Why:** Prompt Arguments are not needed for v0.1
**Revisit:** v0.5 federation layer — maybe add back when implementing task-aware upstream selection.
---

## 2026-05-18 types — ToolExecution removed from v0.1

**Decision:** Removed `ToolExecution` from `ToolDefinition`. Not included in v0.1.
**Why:** Gateway has no task-aware routing in v0.1. Dead fields add maintenance cost with no benefit.
**Tradeoff:** Task support hints are lost during capability indexing. Upstreams with task requirements may be routed to incorrectly.
**Revisit:** v0.5 federation layer — add back when implementing task-aware upstream selection.

---

## 2026-05-18 types — Lean internal structs over spec mirroring

**Decision:** `internal/types` structs contain only fields Heimdall uses for routing, caching, or federation. Passthrough data stays as `json.RawMessage`.
**Why:** Coupling domain types to the full MCP spec creates maintenance burden as the spec evolves. Only own what you use.
**Tradeoff:** Full spec fidelity lost in internal representation. Passthrough fields are opaque bytes.
**Revisit:** Never for the principle. Individual fields added back as gateway features need them.

---