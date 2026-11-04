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
3. Seed a workspace definition with the retired kind. Confirm agent diagnostics reports the load error and leaves the file unchanged.
4. Try the retired config table and get/set/unset paths. Confirm removed-key errors naming `agents.always_on` and the matching suffix. Valid current config paths still work.

The database migration from v0.5.1 is retained; no new migration or runtime rewrite is expected.

Walked 2026-11-04: canonical creation/instance/info and retired create/table/config paths passed.
Workspace load failures are visible in the public agent catalog diagnostics; the ordinary list omits invalid definitions.
The HTTP/UDS owning suite covers update and both duplicate kind locations.
