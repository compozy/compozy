---
id: RT-model-picker-startability-parity
area: RT
title: Model picker offers only models a session can start
persona: Ada
journey: J-session-start
expected: The picker shows only models confirmed available and startable by live discovery, marks the concrete Compozy default model, exposes ACP reasoning levels, and every visible row starts a session; signed-out providers and metadata-only rows remain absent from the picker but inspectable through the catalog API.
entry_points: web model picker; GET /api/model-catalog/models; POST /api/workspaces/:workspace_id/sessions
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits: d4428ab8
evidence: /home/glkifer/dev/qa-labs/compozy-model-picker-startability-20260904-144612-742622-lab/qa-artifacts/qa/evidence
last_report: docs/qa/reports/2026-09-04-model-picker-startability.md
overlaps:
---

With Claude Code reachable, read `GET /api/model-catalog/models?provider_id=claude&view=all` and confirm
every row carrying a fresh `provider_live:claude` source reports `"startable": true`. Start a session on
one of those models and confirm it launches.

Confirm the API marks the model resolved from `providers.claude.models.default` with `"default": true`,
projects the ACP reasoning selector as `"reasoning_source": "acp"`, and the picker renders the concrete
model name with its Default marker and reasoning controls.

Then degrade live discovery: point `providers.claude.command` at a command that never answers, refresh the
catalog, and re-read the list. The same rows must report `"startable": false` with
`"start_blocked_reason": "live_discovery_unavailable"`, remain inspectable through the API, and disappear
from the picker. Repeat with a configured but signed-out provider and confirm its metadata-only rows do
not appear. Confirm no row the picker leaves visible is refused by session start.

Finally, curate one model from settings (toggle `hidden`) and confirm the other curated models keep their
display names, context windows, and prices — a one-model edit must not flatten the rest of the set.

2026-09-04 walk (isolated lab, binary at `d4428ab8`): pass. The cold home reported every curated Claude row
`startable: false` / `live_discovery_unavailable`; a forced refresh made exactly the four advertised models
startable under their logical ids while thirteen `models_dev`-only rows stayed blocked; curating one model
left the other four intact; and a session pinned to `claude-sonnet-5` reached `active`. Browser verification
of the disabled badge was blocked (no browser CLI on the host) and is covered by the selector unit suite
instead. See `docs/qa/reports/2026-09-04-model-picker-startability.md`.

QA impact 2026-09-04: the picker now omits signed-out and metadata-only rows, marks the concrete Compozy
default, and projects reasoning levels discovered from ACP. Reset for API and browser verification.
