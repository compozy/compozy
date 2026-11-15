---
id: RT-session-runtime-live-apply
area: RT
title: Live runtime selection apply on an active session
persona: Bruno
journey: J-21
expected: Changing reasoning effort (or speed or a same-provider model) in an active session_config session's composer sends ACP session/set_config_option immediately, including while a turn is running; the turn keeps running on the same process with runtime.status ready, runtime.effective and transition live_configuration report the new values, a transcript_marker.runtime_changed event records previous/new values and during_turn, and the composer says "Applies at the agent's next step." for next_step providers or "Applies from the next message." for next_turn providers. A refused change keeps the turn, sets runtime.failure, keeps the stored selection, and the next prompt retries it. A provider change waits for the next prompt.
entry_points: web session composer runtime selector; PUT /api/workspaces/:workspace/sessions/:id/runtime; compozy session runtime set
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-072; ET-web-runtime-selector-minimal-slider
---

Added by #1107. Walk with an isolated acpmock lab (set_config_option is observable in the mock's request log) and, when credentials allow, a real Claude Agent Bridge session dropped from xhigh to medium mid-turn.

QA impact 2026-11-15 (#1681): select an advertised model, effort, generic ACP option, or speed
whose provider confirms through a later `config_option_update`. Confirmation during readback or
at the settlement deadline must succeed without an extra set request; a provider that never
confirms must retain the prior effective value and return an unconfirmed rejection. The existing
`TestConfigureRuntime` suite owns the subprocess and deterministic revision-race coverage.
Release mode prohibits local verification; current-main CI owns execution and the live walk remains
unverified. No new CLI/HTTP/native tool shape, config key, hook, or persisted state is introduced.
