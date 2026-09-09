# QA Run Report — 2026-09-09 — PR #560 herdr bridge

Scope: targeted extension runtime and catalog installation. Build: aa5d781 plus PR review remediation.
Environment: macOS, real herdr socket, isolated bridge state and Compozy QA home. No provider prompts required.
Started: 2026-09-09T15:18:00Z. Status: closed.

| Charter | Journey / Scenario | Persona | Tour | Status |
|---|---|---|---|---|
| CH-herdr-bridge-runtime | J-extension-distribution / ET-herdr-bridge-runtime | Ada | Feature | Pass |

## Session debrief

The real hook created pane `w8X:p6`; the public herdr API and bridge status confirmed
its persisted row. A turn-start hook set working, and a stop hook removed its mapping
and closed the pane. The owner and scoped-event APIs served the real session
`sess-9857c879131e8989` in workspace `ws_883e567d315ab5a5`. No inference was launched.
The renderer escaped terminal controls while retaining printable text and whitespace.
No user-facing regressions were observed in this bounded walk.

Evidence: `/Users/pedronauck/dev/qa-labs/compozy-pr-560-herdr-bridge-20260909-151549-489795-lab/qa-artifacts/qa/bridge-runtime.json`.
Bootstrap manifest: `/Users/pedronauck/dev/qa-labs/compozy-pr-560-herdr-bridge-20260909-151549-489795-lab/qa-artifacts/qa/bootstrap-manifest.json`.
Audit: `/Users/pedronauck/dev/qa-labs/compozy-pr-560-herdr-bridge-20260909-151549-489795-lab/qa-artifacts/qa/qa-audit-report.json`.
Teardown: `/Users/pedronauck/dev/qa-labs/compozy-pr-560-herdr-bridge-20260909-151549-489795-lab/qa-artifacts/qa/teardown.json` (required clean before delivery).

Parity: the lab uses a custom daemon socket, so a lab-only `daemon.sock` symlink points
to the manifest listener. The viewer was restarted with the isolated COMPOZY_HOME and
XDG_STATE_HOME, because new herdr shells inherit the operator environment. Session
event retrieval was real; provider inference and loop generation were outside this
non-agent review-remediation journey. Existing user-skill warnings in daemon startup
were unrelated to the bridge. An initial fresh-home session creation required setting
the default provider through the public config CLI; the subsequent creation succeeded.

Supporting verification: eight Python regression tests, production catalog installer,
and `make gate` at `/tmp/compozy-pr-551-560-herdr-20260909/pr-560-gate.log`. The gate classifier now selects catalog
validation and runtime tests; the existing tooling integration suite covers selection.
Cross-surface impact and exact-head CI/merge evidence are recorded in
`/tmp/compozy-pr-551-560-herdr-20260909/pr-560-review-report.md`.

Final status: PASS for the bounded runtime journey; PR delivery additionally requires
the local gate and current-head CI to pass. Zero unresolved behavior findings.

## Follow-up recovery walk

Greptile follow-up findings 3970199538 and 3970199553 are fixed. Complete map saves
now keep a recovery copy; an unreadable primary uses that copy. If both copies are
unavailable, spool payloads remain queued and maintenance reports a nonzero error.
Loop reconciliation saves state under lock, then publishes telemetry after release.
The owning suite now has 11 passing cases, including a real flock availability probe.

The second isolated real-herdr walk damaged a saved primary map, sent another hook,
and observed the same pane ID recover to working before normal terminal cleanup.
Result: PASS. Evidence: `/Users/pedronauck/dev/qa-labs/compozy-pr-560-recovery-20260909-153118-179491-lab/qa-artifacts/qa/recovery-runtime.json`.
Strict audit and clean teardown are recorded alongside that evidence. This walk only
exercises local recovery, with no daemon or provider inference needed. The initial
daemon/owner API walk remains valid for the unchanged session-read boundary.

CI failures on 848a7aedb were README formatting and retired product spelling in the
new package; both were repaired using the repository formatter and language check.
