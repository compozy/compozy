# QA Run Report — 2026-10-17 — Profile-bound session actions

- **Scope:** Issue #783; owner-scoped session actions and bounded subagent polling.
- **Cadence tier:** targeted.
- **Build:** `fix/issue-783-profile-session-404`, working tree initially based on `6de9011b6`, rebased on `3b8302475`.
- **Environment:** isolated bootstrap lab, fresh home, loopback API and Web; no host sessions.
- **Started:** 2026-10-17T23:00:19Z · **Status:** closed (targeted QA; PR delivery tracked separately).

## Persona and flows

Ada manages sessions belonging to a design profile while navigating the aggregate catalog.
She needs to rename, archive, restore, and inspect those sessions without switching the
active profile or losing their history. This walk covers the session-action slice of
`J-scope-work-by-profile`, not its task, automation, billing, or worktree flows.

## Session matrix

| # | Scenario | Persona | Status | Evidence |
|---|---|---|---|---|
| 1 | RT-session-list-row-actions / RT-session-archive-catalog | Ada, desktop | Pass | Isolated browser and API walk below |
| 2 | ET-profile-web-aggregate-owner-surfaces, session slice | Ada, desktop | Pass | Isolated browser and API walk below |
| 3 | ET-web-subagent-card, roster and terminal-error slice | Ada, desktop | Pass | Isolated browser and API walk below |

## Root cause and route audit

The lifecycle and subagent handlers already enforce the selected profile. Web archive,
unarchive, rename, rewind, and subagent cancellation omitted the owner selector, so a
nondefault owner was looked up in the default profile. Roster reads also omitted their
scope. The control poll continued after terminal not-found errors; the cold pagination
effect could repeatedly request a failed next page.

| UI route family | Existing server scope | Resolution |
|---|---|---|
| Detail; subagent list/detail | Explicit profile or authorized aggregate read plus workspace | Preserve aggregate detail reads; use aggregate roster reads for the identified parent. |
| Archive/unarchive, rename | Explicit mutation profile plus workspace | Send captured row owner and workspace; publish the existing selector in OpenAPI. |
| Stop/delete | Explicit mutation profile plus workspace | Existing row-owner propagation retained. |
| Rewind; subagent cancel | Explicit mutation profile plus workspace | Refresh current owner name through canonical detail query; cancel selector documented in OpenAPI. |
| Prompt/cancel, attach, compact, repair, clear, presence, queue, usage, clarifications, interactions, runtime, attachments, derive, transcript/search/outline/stream | Identified session plus workspace; bound session supplies execution context | No default-profile lookup to repair in these routes. Preserve existing handlers. |

The reported presence and third-session detail 404s are not independently reproduced by
the confirmed selector defect. No broad authorization bypass or retry suppression is added.

## Automated evidence

- Targeted Go handler and HTTP/UDS parity suites pass with the race detector.
- Targeted Web adapter, action, control-poll, and subagent wiring suites pass (130 tests across four existing files).
- OpenAPI and TypeScript contracts regenerated through `make codegen`.
- Global gates and full E2E are CI-owned by explicit operator instruction.

## Runtime observations and final status

Targeted behavior verdict: **PASS**. Current-head CI and reviewer delivery remain pending.

- From **All profiles**, renamed a stopped design-owned session, archived it from its
  row menu, then restored it from the session header. PATCH, archive, and unarchive
  returned 200 with `profile=design`; the independent API read confirmed its persisted
  name, owner, stopped state, and cleared archive marker.
- Opened the empty subagent roster: both initial reads returned 200 with
  `all_profiles=true&limit=200`. The subsequent 60.6-second observation window recorded no further
  roster request and no session 404. The exact elapsed interval is retained in the lab's
  `roster-observation.json`.
- Negative API probes reproduced 404 for omitted/wrong owner and 200 for the explicit
  owner or authorized aggregate roster read. They were deliberate probes outside the UI
  observation window.
- Desktop views inspected at 1920×907 and 1280×800. Evidence:
  [archived](../assets/2026-10-17-profile-sessions-archived.png),
  [restored roster](../assets/2026-10-17-profile-sessions-restored-roster.png),
  [1280px roster](../assets/2026-10-17-profile-sessions-roster-1280.png).
- Timer regressions independently verify terminal 404/410 polling stops, 503 recovery,
  and explicit retry after a failed continuation page. No mocked responses were used in
  the real browser walk.
- The promptless stopped session shows the existing composer placeholder "Session failed
  to start". No provider turn was attempted; provider execution, subagent cancellation
  against a live provider, rewind of real messages, and native desktop platforms are not
  claimed by this lifecycle walk. Their affected request wiring is covered by targeted
  suites; provider-wide and platform gates belong to CI.
- React Doctor: 100/100, no issues. Targeted Web typecheck and changed-file lint pass.
  Test-shape checks pass for changed handler/parity files; the spec file has one inherited
  unrelated diagnostic in `TestStopWhenSpecSchemaShouldMatchTheDualWireFormat`.

Lab evidence is retained locally under its bootstrap manifest. Public screenshots include
only disposable QA data. Teardown and the strict evidence audit are recorded below.
No provider inference is required to verify lifecycle and roster behavior for a promptless session.

- Strict targeted QA audit: PASS, zero blockers and warnings.
- Registered lab teardown: `TEARDOWN_ALL_CLEAN=true`; no host daemon/session was changed.

## Review and CI follow-up

- Greptile finding `discussion_r4304958675`: profile IDs are immutable, names are not.
  Rewind, roster-row cancellation, and banner cancellation now force a fresh canonical
  detail read before mutation. Existing hook/wiring cases seed the old cached name and
  verify dispatch uses the renamed owner; one roster-action case covers its separate path.
- CI formatting: split the parity service condition to satisfy the pinned `golines` rule.
- CI exposed a pre-existing timing assumption in the stopping-transition test: asynchronous
  cancellation may advance runtime metadata after request-stop returns. The existing fake
  driver's cancellation hook now holds that I/O boundary until the intermediate memory,
  metadata, catalog, and attention assertions finish. Every assertion remains intact;
  the focused race-enabled case passes 30 repetitions. Production stop behavior is unchanged.
- Post-rebase checks: 112 Web tests in five targeted suites and Web typecheck passed.
  Owner-refresh follow-up: 54 tests across the two affected suites, Web typecheck, and React Doctor (100/100) passed. Final remote status is recorded in the PR.
