---
title: Dry-run proves the run you are about to submit
type: fix
---

A Loop could validate, dry-run cleanly, and then fail at submission with `executed definition template manifest changed`. The compiler folded default values into the definition it stored, but compiled templates from the definition _before_ those defaults — so a persisted run carried more template keys than its own snapshot, and hydration rightly refused it. Compilation now uses one canonical definition throughout, and dry-run exercises the exact snapshot boundary a real submission uses. (#313, #317)

- Defaults are folded once at the start of compilation and used for linting, contracts, nodes, watch events, graph metadata, and child Loops alike — so omitted child `mode` values no longer appear out of nowhere during hydration.
- A snapshot must load through the production loader before its bytes or digest are returned; one that cannot round-trip can no longer reach storage.
- `compozy loop run --dry-run` and `compozy__loop_run` with `dry: true` run that same check, so a preview can no longer approve a definition that submission would reject.
- A mismatch now names the manifest kind, the exact key, and its source instead of reporting a generic failure.

Migration notes: no storage, API, CLI, or configuration contract changed. Integrity checks were not relaxed — inconsistent definitions are still rejected, just earlier and with a readable reason.
