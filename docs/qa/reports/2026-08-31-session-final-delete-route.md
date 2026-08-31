# QA Run Report — 2026-08-31 — session-final-delete-route

- **Scope:** Packaged web-assets synchronization for the empty Sessions route reached after deleting the final session
- **Cadence tier:** targeted
- **Build:** `98e4aa7c81f0726916c01c4a08d153d6b924be69` with local web-assets pin fix · **Environment:** isolated production-bundle daemon and Chromium
- **Started:** 2026-08-31T18:55:00-03:00 · **Status:** closed at 2026-08-31T19:24:40-03:00

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | CH-untested-016-15-theo |

## Flows in Scope

- `J-15` — A returning session user deletes the final session and remains in a usable empty Sessions window (`../journeys/J-15-operate-session-via-cli-api.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-untested-016-15-theo | J-15 / RT-014 | Théo | Feature Tour | Pass | compozy/compozy#511 | this change |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-untested-016-15-theo — Théo

- **Ran:** 2026-08-31T18:55:00-03:00 → 2026-08-31T19:00:00-03:00 (box respected: yes)
- **Findings:** The corrected production bundle kept the user inside the Sessions window after deleting the final session. No new finding was observed.
- **Bugs filed/updated:** The pre-existing product finding is tracked in `compozy/compozy#511`; no new QA-registry symptom was found during this walk.
- **Scenarios settled:** RT-014 → pass
- **Paper cuts:** None.
- **Surprises:** The bootstrap did not detect the ephemeral `npx agent-browser` fallback, but Chromium executed the required Web journey and produced evidence successfully.
- **Suggested next charter:** Re-run the existing session catalog charter when the next packaged release candidate is available.

## What Was Fixed

### compozy/compozy#511: Packaged UI omitted the empty Sessions route

- **Symptom:** Deleting the final session navigated to `/sessions`, where the packaged UI rendered `Page not found`.
- **Root cause:** `go.mod` pinned `compozy-web-assets@v0.0.190`, generated before `web/src/routes/_app/sessions.tsx` existed.
- **Fix:** Update the pin to `v0.0.196`, the published deterministic bundle for source commit `98e4aa7c81f0726916c01c4a08d153d6b924be69`.
- **Regression test:** `webAssetsCheck` failed before the pin update with digest `e2b1ac…` versus `3c929e…`, then passed after the update. The daemon-served browser replay also passed after deletion and refresh.
- **Retested:** RT-014 in a fresh isolated daemon plus the adjacent empty-catalog read through CLI and HTTP.

## Paper Cuts

None.

## Runtime Errors Observed

The expected pre-fix `webAssetsCheck` digest mismatch was captured during the red phase. No runtime or browser error appeared in the corrected walk.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- The navigation coordinator and source route were already correct; release-bundle coherence was the responsible boundary.
- The existing full-bundle digest check is stronger and less brittle than a new assertion against minified JavaScript text.
- Production-parity deviation: `agent-browser` ran through `npx` because the CLI is not installed globally; it used the host-provided `/bin/google-chrome` and did not download a browser.

## Final Status

- **Exit gate (full automated suite):** PASS — `make gate`; `go-lint` and `go-test -race ./...` have current evidence records for the finalized report tree.
- **Gate log:** `/home/francisross/tmp-builds/compozy-session-final-delete-route-20260831-215333-848220-lab/qa-artifacts/qa/logs/final-make-verify.log`.
- **Additional frontend evidence:** PASS — `make bun-lint`, `make bun-typecheck`, and `make bun-test`.
- **Embedded boundary evidence:** PASS — `webAssetsCheck` and `CGO_ENABLED=1 go test -race ./internal/api/httpapi -run '^TestStaticRoutesServeEmbedded(IndexForRootAndDeepLinks|Assets)$' -count=1`.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journeys walked; Web, CLI, and HTTP surfaces confirmed. UDS is represented by the installed CLI transport against the isolated socket.
- **Verdict:** PASS — ready for review; the corrected packaged bundle resolves `/sessions` after the final session is deleted.
