---
name: cy-web-docs-impact
description: "Audit backend tasks/spec coverage for Web, site, public agent interfaces, config, and extensibility impact. Use for contract, handler, CLI, extension/hook/tool/registry/bridge/MCP/workflow changes; excludes refactors without those impacts."
trigger: explicit
argument-hint: "[task-or-spec-path]"
---

# Change Impact

Resolve the requested spec/task/diff from context and use `docs/_memory/change-impact.md` as the canonical impact contract. Inspect code to identify affected files when the task has not enumerated them; missing enumeration alone is not a reason to ask the user to do the research.

Record one owning analysis covering changed Web/site, agent CLI/HTTP/UDS, native tools, extension/hook/resource/registry/bridge/MCP, config lifecycle, and workspace isolation surfaces. Name concrete paths/contracts and their verification owner. An unaffected surface gets a short reason; dependent tasks link to the analysis and update deltas instead of repeating full subsections. Each linking task states its own delta against the analysis, or `no delta` plus the surfaces it checked, so a stale link is visible rather than silent.

A user-visible capability with no planned agent-operable path (CLI verb, HTTP/UDS route, structured output, deterministic error contract) is a design finding to surface before the task shape settles, unless the spec records the exception and its reason.

Read `references/audit-triggers.md` for a relevant boundary trigger. Public removals and config changes follow SD-013 deprecation/migration, not unconditional hard cuts. Update documented paths from the actual repository; do not invent a system module because an API exists.

Contract changes co-ship codegen and affected client/docs checks: `make codegen`, `make codegen-check`, and Turbo validation from the repo root under the root gate policy. User-visible changes identify affected QA scenarios and their walk owner; editorial/internal changes do not create a lab.

Resolve ordinary shared ownership from the task graph and existing owners. Escalate only an unresolved product contract or authorization decision, not a missing template phrase.
