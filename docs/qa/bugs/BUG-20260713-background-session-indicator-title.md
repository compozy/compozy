# BUG-20260713-background-session-indicator-title: Live sessions lose identity and visibility across workspace switches

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-11 return to a running session, steps 2–4
- **Scenarios:** RT-workspace-active-session-badge; RT-session-auto-title
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** Linear AGH-84

## Summary

Théo left a live Cursor/Grok turn running in the launch workspace and switched to `bench-ops`. The turn continued and its transcript was intact on return, but the workspace switcher showed no active count or running signal for the owning workspace after the one-time redirect toast disappeared. After two substantive real-provider turns and multiple workspace navigations, the session was still named `general` / `sess-b1c980b...` instead of receiving a useful automatic title.

## Reproduction

- **Charter:** CH-background-session-switch · **Tour:** Interrupt Tour
- **Environment:** desktop / wifi-fast / en-US; two isolated local workspaces; live Cursor/Grok 4.5 session.

1. Start a real task in `agh-automation-features-...-lab` and wait for the session to show `running`.
2. Switch to the registered `bench-ops` workspace while the turn is in flight.
3. Wait for the redirect toast to expire and inspect both workspace buttons and the scoped Agents count.
4. Return to the owning workspace, reopen the session, and inspect the transcript and title after completion.

**Expected:** The non-active workspace button shows the exact live-session count/state, provides a durable return affordance, and the first meaningful task generates one concise title used consistently in topbar and lists.
**Actual:** Background execution survived and the owning workspace later showed `Agents 1/4`, but the neighboring workspace exposed no persistent signal after the toast expired. The title remained the generic agent name/raw session id after two completed turns.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-background-session-no-workspace-indicator.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-background-session-no-persistent-indicator.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-new-session-grok-transcript.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-final-replay-20260713-20260713-194432-535561-lab/qa-artifacts/qa/screenshots/rt-agh84-onboarding-session-counted-as-user.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/rt-agh84-onboarding-system-badge-one-fixed.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/rt-agh84-cross-workspace-return-fixed.dom.txt`
- Session `sess-b1c980b86709053d` completed the background turn in 9 seconds and retained the full transcript.

## Fix

- **Root cause:** Confirmed at three independent owners. The session manager never persisted a meaningful `Name` for an unnamed user session after its first authored task, so topbar/catalog fallbacks exposed the agent name or raw ID. Separately, the shell queried only the selected workspace and had no workspace-scoped session-catalog lifecycle wake, so it could not reconcile exact inactive-workspace user-session counts. Public catalogs originally lacked an exact session-type filter; the final replay then proved that filtering is insufficient while the onboarding wizard creates its internal implementation session as `type=user`.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** Manager ownership covers one persisted automatic title after the first meaningful user task. `internal/session/query_test.go` now proves at the authoritative creation boundary that the managed onboarding agent is persisted as `system`, a normal agent remains `user`, `type=user` returns exactly the normal session, and the all-type operational page retains both. HTTP/UDS/CLI/native-tool catalog suites cover exact `type=user`, workspace-scoped wakes, durable removal, and reconnect snapshots. Web route/layout/cache/stream suites cover per-workspace isolation, exact totals, return links, title fallbacks, and internal-session exclusion. The new correction passed its directed red/green, the complete `internal/session` race suite, and the canonical four-test Web onboarding hook suite; final live replay remains pending.

## Verification

- Same-persona live replay passed in a fresh post-fix lab with real Cursor/Grok 4.5. Onboarding session `sess-8101d12c9aaa4db0` was persisted as `system`; the all-type active catalog retained it while `type=user` returned only the one authored session and the inactive `agh3` rail displayed exact count `1`. Two user sessions then received durable automatic titles and reciprocal Return links across `agh3` and `bench-ops`; both direct session-to-session returns converged to the correct URL, selected workspace, banner, and transcript without a Loading loop. Stopping the bench session removed its badge immediately, deleting it through the confirmation modal reset Sessions to `0`, direct read returned 404, and the user-only active catalog returned to total `1`. System onboarding exclusion is therefore verified from creation through UI aggregation; task-role and Goal-judge exclusions remain owned by their dedicated scenarios.

## Re-found (2026-07-13)

In the fresh final-replay lab, onboarding created internal Cursor session `sess-cdd8a43c9902d4be` but persisted it as `type=user`. After Théo created one real user session and switched to `bench-ops`, the inactive `agh3` rail reported `2 active sessions` instead of `1`. A fresh public session-catalog read independently returned the same two `type=user` rows. The internal-session-exclusion branch is therefore reopened at the creation/classification boundary; the title and durable return-link branches remain accepted.

The serialized GPT-5.6-SOL/high correction now reconciles the final session type after authoritative agent resolution and before lineage, metadata, and catalog persistence. `onboarding` is the only managed internal agent name and now becomes the existing `system` class; ordinary user agents preserve their requested normalized type. The fresh onboarding replay proved the visible count and public catalog boundary, so this re-found branch is verified.
