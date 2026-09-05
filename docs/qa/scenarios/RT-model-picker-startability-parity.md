---
id: RT-model-picker-startability-parity
area: RT
title: Model picker offers only models a session can start
persona: Ada
journey: J-session-start
expected: Every Claude row the picker leaves enabled starts a session; when live discovery is degraded, those rows render disabled with a stated reason, the provider default still starts, and no enabled row is refused at session start.
entry_points: web model picker; GET /api/model-catalog/models; POST /api/workspaces/:workspace_id/sessions
qa_status: pass
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

Then degrade live discovery: point `providers.claude.command` at a command that never answers, refresh the
catalog, and re-read the list. The same rows must now report `"startable": false` with
`"start_blocked_reason": "live_discovery_unavailable"`, the picker must render them disabled with a stated
reason rather than hiding them, and the provider default must still start a session. Confirm no row the
picker leaves enabled is refused by session start.

Finally, curate one model from settings (toggle `hidden`) and confirm the other curated models keep their
display names, context windows, and prices — a one-model edit must not flatten the rest of the set.

2026-09-04 walk (isolated lab, binary at `d4428ab8`): pass. The cold home reported every curated Claude row
`startable: false` / `live_discovery_unavailable`; a forced refresh made exactly the four advertised models
startable under their logical ids while thirteen `models_dev`-only rows stayed blocked; curating one model
left the other four intact; and a session pinned to `claude-sonnet-5` reached `active`. Browser verification
of the disabled badge was blocked (no browser CLI on the host) and is covered by the selector unit suite
instead. See `docs/qa/reports/2026-09-04-model-picker-startability.md`.
