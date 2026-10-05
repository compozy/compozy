# BUG-20261004-palette-lists-closed-clients: Closed tabs remain command destinations

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-command-palette, discover and target an attached client
- **Scenarios:** ET-agent-command-invoke
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Ada cannot trust the advertised command destinations after Bruno closes a browser tab. The closed
tab remains listed, selecting it fails, and automatic targeting can demand a choice among clients
that no longer exist.

## Reproduction

- **Charter:** CH-palette-approval-exactly-once · **Tour:** Interrupt Tour, attached-client leg
- **Environment:** isolated production daemon at port 50727; Bruno controls two desktop Chrome
  clients at 1512×862, en-US, and Ada owns independent structured reads and invocation.

1. Open two tabs in Studio Operations and discover their IDs through the public client list.
2. Close the observer tab while keeping the origin responsive.
3. Read GET /api/cmd-palette/clients?workspace=ws_7af64cef6bc02b2b over UDS.
4. Invoke palette.view.profiles explicitly on the closed client.

**Expected:** the closed client disappears from the targeting list and is refused as unattached.
**Actual:** web-ee953872-3bf1-440b-b9b6-e2f58e47027b remains among 16 listed clients; targeting it
returns 503 runtime_unavailable. Fourteen earlier closed clients were already present before
this replay. The remaining origin is responsive and no profile mutation occurs.

## Evidence

- docs/qa/evidence/2026-10-02-untested/profile-client-disconnect-close-observer.json
- docs/qa/evidence/2026-10-02-untested/profile-client-disconnect-listed-after-close.json
- docs/qa/evidence/2026-10-02-untested/profile-client-disconnect-invoke-closed.json
- docs/qa/evidence/2026-10-02-untested/profile-client-lifetime-before-replay.json
- docs/qa/evidence/2026-10-02-untested/profile-navigation-two-clients-e2e-current-route.json:
  all three E2E-027 repetitions complete navigation and then fail because the closed observer
  remains in the advertised inventory.
- Recording profile-navigation-fixed-bruno is stopped at 25 frames; the persona session ends
  before source investigation.

## Fix

- **Root cause:** the palette directory lists registered WindowManager views. Registration survives
  command-stream disconnection, while command dispatch uses the separate active endpoint map.
  These lifetimes disagree. Registration must remain available for context and reconnect authorization.
The directory now reads active command endpoints through the existing WindowManager registry.
Registered views remain the authority for context and reconnect authorization. No timer,
best-effort unload request, registration deletion or swallowed disconnection error is used.

- **Fix commit:** 259d7142c
- **Regression test:** existing IT-031 in internal/daemon/cmd_palette_integration_test.go;
  existing E2E-027 in web/e2e/__tests__/profiles.spec.ts provides real-tab lifecycle coverage.

## Verification

IT-031 fails before repair and passes with race detection after repair. E2E-017 and E2E-027 each
pass three real-daemon repetitions. A fresh Ada/Bruno replay confirms two-client ambiguity,
explicit targeting, removal after close, automatic targeting of the survivor, reconnect with
the same identity, and zero-client refusal after closing both tabs. The first immediate
post-reload read observes the disconnected interval; the later independent read and invocation
prove recovery without another reload. The screenshot is inspected and the 11-frame recording
is stopped. Receipts: palette-client-lifetime-fixed-*.json, integration-{red,green}.json and
real-e2e.json in the cycle directory. Production repair: 259d7142c. The gate passed before commit; a hook-only formatter disagreement in the integration fixture is being normalized separately. The wider
structured invocation/approval/native matrix is not settled by this adjacent lifetime probe.
