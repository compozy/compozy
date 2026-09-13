# BUG-20260913-marketplace-background-refresh: Expired catalog remains stale after a successful background refresh

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** Open Marketplace with an expired or first-use catalog
- **Scenarios:** ET-web-marketplace-landing-browse
- **Found:** 2026-09-13 · **Report:** reports/2026-09-13-marketplace-catalog.md

## Reproduction

Open the catalog after its TTL expires. The first read starts an asynchronous daemon refresh but
returns the old snapshot. The Web keeps that snapshot indefinitely until another user action causes
a read. A fresh catalog can stay empty; a cached one says the sources did not answer even after
the public endpoint returns healthy sources and newer last_read_at values.

## Root Cause and Repair

The API mapped every stale snapshot to degraded, including a healthy snapshot whose TTL expired.
Degraded now requires a recorded source failure; never-read sources retain never. The Web follows
the daemon-owned `refreshing` flag once per second until all pending sources settle, independently
of errors from other sources. First-read pending catalogs show loading; an expired healthy snapshot remains visible with
a refreshing status. Recorded failures keep the existing cached/error/Retry behavior.

Invariant/owner: TestMarketplaceCatalog owns healthy-expired versus failed HTTP/UDS projection.
The existing use-marketplace query lifecycle suite owns atomic replacement after pending refresh.
Both regressions failed before production repair. Core race passed1.143s; the67owning Web tests
passed9.152s, preserving failure retry and cached-data assertions. Live production re-walk passed: after expiring the lab TTL, the browser made two successful catalog reads, displayed14rows and removed the stale status without manualRefresh. The lab TTL was restored to1h. Evidence background-refresh-after.json/png.

Evidence: .cache/marketplace-task10-stale-{core-red,query-red,core-green,web-green}.log.

Final review found the mixed-source variant: a recorded failure stopped observation of another pending source. The repair now exposes existing flight state rather than inferring it from error absence. Completion during a SQLite snapshot read requests one final read. New owning evidence is recorded in reviews-001/issue_001.md; the earlier browser evidence remains the single-source walk.
