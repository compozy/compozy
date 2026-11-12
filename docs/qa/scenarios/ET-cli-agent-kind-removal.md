---
id: ET-cli-agent-kind-removal
title: Create always-on agents after removing retired kind inputs
area: ET
qa_status: untested
last_report: reports/2026-11-04-agent-kind-removal.md
---

As a CLI operator upgrading beyond v0.5.2:

1. Create `triage` with `compozy agent create triage --kind always_on --prompt ...`, create its instance, then inspect `compozy agent info triage -o json`. Confirm `kind: always_on` using a fresh public API read.
2. Run create/update/duplicate with the retired kind. Confirm exit 1 and the actionable `agent_kind_invalid` error.
3. Seed an AGENT.md with the retired kind in each discovery root, including a package. Confirm discovery rejects it with `agent_kind_invalid` and leaves every byte unchanged.
4. Load the retired agent-kind config table from each config scope. Confirm rejection with `agents.always_on` guidance and unchanged source bytes. Canonical definitions and config still load.

The v0.5.1 database migration remains. v0.6.0 is the intermediate release for lossless file migration; v0.6.1 requires canonical files. Current-head CI owns automated verification; no local walk is authorized for this release push.

Walked 2026-11-04: canonical creation/instance/info and retired create/table/config paths passed.
Workspace load failures are visible in the public agent catalog diagnostics; the ordinary list omits invalid definitions.
The HTTP/UDS owning suite covers update and both duplicate kind locations.
