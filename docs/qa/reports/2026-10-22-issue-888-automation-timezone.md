# QA Run Report — 2026-10-22 — Automation time zone

- **Scope:** Issue #888: first-config host-zone detection and Settings time zone selection.
- **Cadence tier:** targeted
- **Build:** assigned issue branch; exact commit and CI evidence recorded in the delivery PR.
- **Environment:** isolated daemon home and ports; real local CLI/API/Web, no agent execution. Provider-usage collection was disabled during setup.
- **Status:** pass (targeted behavioral QA and affected local gate); PR CI and reviews pending

## Session matrix

| Session | Persona / Tour | Scenario | Status |
| --- | --- | --- | --- |
| First-config and compatibility | Cora / Locale | Fresh config, UTC fallback, existing config preservation | Pass |
| Time zone picker | Cora / Feature | TA-web-automations-shell-entry: search, save, reload, mismatch, machine shortcut | Pass |
| Schedule consistency | Cora / Locale | CLI config parity and 08:30 schedule preview/next run | Pass |
| Input and viewport edges | Cora / Feature | Invalid API zone, keyboard selection, no matches, narrow viewport | Pass |

## Charter

Cora schedules a morning briefing in Sao Paulo. Enter through Settings → Automation, discover the
UTC mismatch, search for a zone, save it, reopen the page, and independently read config through the
CLI. Use the host-zone shortcut and confirm the morning schedule reflects the saved zone. Probe
keyboard search, an unmatched search, a narrow viewport, and invalid-zone rejection. Adjacent
canary: the Manage row still opens Automations. This targeted session does not exercise provider
execution or unrelated dock/palette/Loop flows.

## Evidence and debrief

- The real CLI persisted `America/Sao_Paulo` for a new config and explicit `UTC` for invalid host
  detection. Existing empty configs stayed implicit UTC; an explicit London zone survived reinstall
  in the owning config suite.
- Web search selected Tokyo. Save/reload and independent CLI readback agreed. With the browser
  emulating Berlin, the shortcut selected the daemon's Sao Paulo zone and removed the mismatch.
- Keyboard search selected Paris; Discard retained the saved zone. No-match input did not alter it.
  The 390 × 844 viewport kept the picker usable without horizontal overflow. Manage opened Automations.
- An invalid-zone Settings PATCH returned 400 and left config unchanged. A daemon started with invalid
  host `TZ` remained healthy, omitted `host_timezone`, and left the picker usable without the shortcut.
- The real walk reproduced a pre-existing stale cron cursor after a zone edit/restart. The owning
  SQLite integration regression failed before the fix. It now passes for zone changes, unchanged
  zones, legacy future/catch-up cursors, interval/timestamp invariance, and deferred reservations.
  A legacy unstarted reservation is also retained during identity upgrade.
- Targeted Web route tests pass (including the fixed-offset regression); Web typecheck/build pass through root Turborepo. React
  Doctor found no issues in the final changed-file scan. Config, Settings, API conversions and existing DST scheduler checks pass
  with the race detector. The additive OpenAPI semantics check passes with only `host_timezone`
  added to the approved historical contract.
- Screenshots: `timezone-mismatch.png` (1440 × 900), `timezone-narrow.png` and
  `timezone-search-narrow.png` (390 × 844), retained in the isolated lab's QA artifacts. Public
  R2 links are attached to the PR after the implementation commit.

The repaired binary re-walk passed: the same persisted 08:30 cron moved from `2026-10-23T08:30:00Z`
to `2026-10-23T11:30:00Z` after the Web shortcut, save, and isolated daemon restart. A second restart
without changing the zone preserved `11:30Z`. The Automations sentence remained 08:30 Sao Paulo
and both cron rows agreed on their next run. The affected `make gate` passed all lanes, including 687 Web test files / 7,264 tests.
The isolated lab teardown returned `clean=true`, including its daemon, browser, launcher, and
named browser automation process.

## Review remediation

Greptile identified that the browser's standard time zone list omits valid IANA choices such as
`Etc/GMT+3`. The existing route suite reproduced the failure before the production fix. Search now
resolves valid typed IANA names into selectable choices, while invalid names and bare numeric offsets
leave the current selection unchanged. The daemon retains final validation when saving.

The real app re-walk searched and selected `Etc/GMT+3`, saved it, reloaded the page, and independently
confirmed the persisted value through the CLI. `timezone-fixed-offset-search.png` captures the
rendered result. Web build/typecheck and the final React Doctor scan passed.

## Limits

No local E2E suite or gate-full is run; CI owns those gates. Native Windows zone-name detection is
not mapped to IANA: unrecognized host names fall back to UTC. Other platforms are checked with
filesystem fixtures; only macOS receives a live app walk in this run.

## Final status

PASS for the targeted behavioral scope and affected local gate. PR delivery remains in progress
until current-head CI and both requested reviewers are complete.
