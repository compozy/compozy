---
id: ET-cli-agent-kind-removal
title: Create always-on agents after removing retired kind inputs
area: ET
qa_status: pass
last_report: reports/2026-11-04-agent-kind-removal.md
---

As a CLI operator upgrading beyond v0.5.2:

1. Create `triage` with `compozy agent create triage --kind always_on --prompt ...`, create its instance, then inspect `compozy agent info triage -o json`. Confirm `kind: always_on` using a fresh public API read.
2. Run create/update/duplicate with the retired kind. Confirm exit 1 and the actionable `agent_kind_invalid` error.
3. Seed a workspace definition with the retired kind. Confirm discovery loads it as `always_on` and safely updates only the kind value when writable. Read-only files remain readable without edits.
4. Load a persisted retired config table and AGENT.md from each discovery root. Confirm migration, canonical precedence, preserved conflicts and idempotence. Try retired get/set/unset paths and confirm errors naming `agents.always_on` and the matching suffix. Valid current config paths still work.

The database migration from v0.5.1 is retained; persisted-file migration runs at the boundary loader through v0.6.0 and is scheduled for removal in v0.6.1.

Walked 2026-11-04: canonical creation/instance/info and retired create/table/config paths passed.
Workspace load failures are visible in the public agent catalog diagnostics; the ordinary list omits invalid definitions.
The HTTP/UDS owning suite covers update and both duplicate kind locations.
