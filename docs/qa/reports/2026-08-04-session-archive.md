# QA Run Report — 2026-08-04 — session archive

- **Scope:** Durable session archive management, session-row action menus, and normal/archived catalog separation across Web and structured surfaces
- **Cadence tier:** targeted
- **Build:** e40dc76 · **Environment:** isolated local daemon, current-source Vite Web, and deterministic Storybook captures
- **Manifest:** `/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Started:** 2026-08-04T22:12:17-03:00 · **Status:** PASS

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Cora | Casual User | laptop / wifi-fast / en-US | CH-archive-session-catalog |
| Ada | Power User | desktop / wifi-fast / en-US | CH-archive-session-structured-parity |
| Nia | New User | laptop / wifi-fast / en-US | CH-session-row-open-canary |

## Flows in Scope

- `J-archive-session-without-deleting` — Hide finished work without losing its history (`../journeys/J-archive-session-without-deleting.md`)
- `J-12` — Open a session normally after the shared row gains an actions target (`../journeys/J-12-open-session-fast.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-archive-session-catalog | J-archive-session-without-deleting / RT-session-list-row-actions | Cora | Feature Tour | Pass | | |
| 2 | CH-archive-session-catalog | J-archive-session-without-deleting / RT-014 | Cora | Feature Tour | Pass | | |
| 3 | CH-archive-session-catalog | J-archive-session-without-deleting / RT-082 | Cora | Feature Tour | Pass | | |
| 4 | CH-archive-session-catalog | J-archive-session-without-deleting / ET-web-sessions-catalog-modal | Cora | Feature Tour | Pass | | |
| 5 | CH-archive-session-structured-parity | J-archive-session-without-deleting / RT-session-archive-catalog | Ada | Feature Tour | Fixed | BUG-20260805-session-archive-sdk-missing; BUG-20260805-archived-detail-unarchive-missing | e40dc76 |
| 6 | CH-archive-session-structured-parity | J-archive-session-without-deleting / RT-011 | Ada | Feature Tour | Pass | | |
| 7 | CH-session-row-open-canary | J-12 / RT-012 | Nia | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Cora — CH-archive-session-catalog

Cora opened the live global catalog, used the three-dot menu without triggering row navigation, and
confirmed the correct active, stopped, and archived actions. Archive and Unarchive moved the same row
between the normal and collapsed Archived groups. Delete still required confirmation, Cancel kept the
row, Escape returned focus to the trigger, and desktop/narrow layouts remained usable.

### Ada — CH-archive-session-structured-parity

Ada archived and restored the same stopped session through CLI, HTTP, UDS, native tools, and a live
installed extension. Default, archived-only, and inclusive lists remained exact and disjoint. Active
archive attempts returned a conflict, archived prompt/resume/attach attempts were rejected, repeated
operations were idempotent, the marker survived daemon restart, and a second workspace saw only
not-found responses.

### Nia — CH-session-row-open-canary

Nia opened a normal session from the row's dedicated navigation target and reloaded the canonical
route successfully. A direct archived route remained readable with history intact, showed the
Archived marker, blocked runtime writes, and offered Unarchive rather than Attach or Resume.

## What Was Fixed

- `BUG-20260805-session-archive-sdk-missing` — added typed Go/TypeScript Host API helpers and preserved archive operations through the daemon adapter. Red-before/green-after coverage lives in the public SDK suites and `TestNewHostAPISessionManagerAdapter`; the live extension replay passed.
- `BUG-20260805-archived-detail-unarchive-missing` — wired the existing unarchive mutation into archived session page controls and made it the topbar lifecycle action. The topbar regression test failed before the fix and passed after; the live detail replay passed.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Cora | Archived detail | Restoring required returning to a catalog | Sharp | Fixed and verified as BUG-20260805-archived-detail-unarchive-missing |
| Ada | Extension Host API | Typed archive helpers and adapter forwarding were missing | Sharp | Fixed and verified as BUG-20260805-session-archive-sdk-missing |

## Runtime Errors Observed

- Expected validation responses: 409 for archiving an active session; 400 for an invalid archive filter; 409 for prompt/resume/attach while archived; 404 for cross-workspace reads and writes.
- The initial extension internal error was captured, fixed, and replaced by a successful full Host API round trip. No unexplained daemon, Web console, or transport error remained.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Archive is catalog metadata, not a lifecycle state: stopped runtime truth and reversible catalog visibility remained independent across every surface.
- Generated Host API contracts do not guarantee ergonomic SDK parity or adapter capability preservation; both require explicit regression coverage.
- Removing ineligible detail actions must pair with the next valid recovery action, otherwise a readable archived session becomes a navigation dead end.
- The daemon serves released embedded Web assets, so current-source UI verification used the isolated Vite proxy target from the QA manifest. Runtime/API behavior remained production-like; visual contracts used deterministic Storybook captures.

## Automated Exit Gate

`make gate` classified the schema, OpenAPI, Go, SDK, and Web changes as a full-gate escalation and
completed `make verify` successfully after the QA fixes. Codegen, installer checks, source-size
limits, zero-warning lint, typecheck, unit/race tests, Web build, Go build, and boundaries all passed.
Final make verify evidence: `/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/final-make-verify.log`

## Evidence

- Journey log: `/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/journey-log.jsonl`
- Structured archive round trip: `/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/extension-host-api.json`
- Restart persistence: `/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/daemon-restart.json`
- Visual captures: `/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/screenshots/`
- Strict evidence audit: `/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/qa-audit-report.md`
- Clean teardown: `/Users/pedronauck/dev/qa-labs/compozy-session-archive-20260805-031044-743468-lab/qa-artifacts/qa/teardown.json`

## Final Status

Behavior is ready: all seven in-scope scenarios are terminal, both Friction findings are fixed and
verified, and the automated exit gate passed. Verdict: PASS. Clean process teardown is recorded in
the isolated lab with `clean: true`.
