# Issue #832: session metadata and aggregate folders

## Scope and charter

Bruno manages two profiles in one workspace. He identifies every session by runtime and agent,
organizes work in folders, switches between a profile and All profiles, renames a folder belonging
to the other profile, and reloads to confirm saved membership. The adjacent canary is keyboard
session cycling through collapsed folders. Only targeted checks and isolated runtime/browser
journeys are authorized; CI owns global delivery gates.

## Session matrix

| Scenario | Status | Evidence |
| --- | --- | --- |
| ET-web-session-folders-manual-order | Pending | |
| ET-profile-web-aggregate-owner-surfaces | Pending | |
| ET-profile-aggregate-owner-labels | Pending | |
| RT-session-list-row-actions | Pending | |

## Lab

Manifest: `/Users/pedronauck/dev/qa-labs/compozy-issue-832-session-list-20261019-202236-484729-lab/qa-artifacts/qa/bootstrap-manifest.json`.

## Checks

- `CGO_ENABLED=1 go test -race ./internal/api/core -run '^TestSessionPayloadFromInfo$' -count=1`: passed.
- `make codegen`: passed; additive session runtime contract and generated clients co-ship.
- Frontend owning suites: running.

## Session debriefs

Not walked yet.

## Final status

In progress. No delivery claim.
