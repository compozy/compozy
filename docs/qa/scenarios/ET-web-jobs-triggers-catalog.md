---
id: ET-web-jobs-triggers-catalog
area: ET
title: Jobs and Triggers catalog plus deep-linkable detail
persona: Bruno
journey: J-24
expected: `/jobs` and `/triggers` render as ListingPage catalogs (PageHead + ListingToolbar search/filters/view + rows/cards) instead of SplitPane master-detail; row click opens `/jobs/$jobId` or `/triggers/$triggerId` with breadcrumb parent link; Create CTA stays in topbar actions; `?create=loop&loop=` from Loop detail still opens the create editor seeded at that Loop; dynamic Edit/Delete remain source-gated detail actions, and Run now is available only for Jobs.
entry_points: web `/jobs`; web `/triggers`; Loop detail Add schedule/trigger CTAs
qa_status: pass
bug_ids: BUG-20261004-automation-offline-pagination-silent; BUG-20261003-background-windows-forget-global; BUG-20260713-workspace-trigger-loop-submit-inert; BUG-20261003-loop-mapping-example-rejected; BUG-20261003-webhook-sample-invalid-json
fix_status: fixed
retest_status: pass
fix_commits: 78133b4f0f2c477f7fbc48b9abe1678463c0c5d0
evidence: docs/qa/evidence/2026-10-02-untested/trigger-recovery-bruno-ended.json; docs/qa/evidence/2026-10-02-untested/catalog-recovery-replay-bruno-ended.json; docs/qa/evidence/2026-10-02-untested/catalog-recovery-replay-global-failed-history.json; docs/qa/evidence/2026-10-02-untested/catalog-recovery-replay-jobs-offline-observed.json; docs/qa/evidence/2026-10-02-untested/catalog-recovery-replay-triggers-offline.json; docs/qa/evidence/2026-10-02-untested/catalog-recovery-replay-pending-route-node.json; docs/qa/evidence/2026-10-02-untested/catalog-recovery-replay-trigger-route-back.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: TA-052; TA-056; TA-automation-crud-loop-target; LP-033
---

Added by Route Chrome + catalog migration (2026-07-17). Selection moved from local state to detail child routes.

2026-09-10 CI regression: With Global on and no project selected, triggering a failing global
job must leave its detail window and URL open even when the job registers a workspace. Reload
must keep the detail and show the failed run with browser/CLI diagnostics. Verification owner:
the existing failure-diagnostics case in `web/e2e/__tests__/jobs-hardening.spec.ts`.

QA impact 2026-07-18: when a refresh or later page fails after rows loaded, the catalog preserves
the rows and shows the query failure instead of presenting stale data as a successful refresh.

QA impact 2026-07-18: concurrent Run now actions keep each job disabled until its own request
settles, and a repeated click cannot queue duplicate work for a job already in flight.

QA impact 2026-07-18: changing a detail route ID resets queued-run and editor-local state before
the next Job or Trigger renders. A consumed Loop create seed can be cleared and a later seed opens
again without remounting the catalog.

QA impact 2026-07-18: a runtime-disabled or 503 catalog may retain cached Jobs, but every manual
Run now control in rows, cards, and Job detail remains disabled and the hook refuses dispatch.

QA impact 2026-07-18: the one-shot `?create=loop&loop=` handoff now removes the Loop seed from
both route-loader dependencies and mounted catalog filters before list queries run. The editor
still opens with the requested Loop while the Jobs/Triggers catalog remains unfiltered. Status
remains untested; no QA replay ran.

QA impact 2026-07-18: a failed background detail refetch now preserves the cached Job or Trigger
panel instead of replacing usable data with a route-level error. Status remains untested; no QA
replay ran.

QA impact 2026-07-18: workspace-scoped Job and Trigger details, histories, editors, and mutations
are withheld unless their `workspace_id` exactly matches the active workspace; changing workspaces
also closes an open editor. Managed-detail guidance now distinguishes config-owned automation from
package-provided automation instead of describing both as configuration files.

QA impact 2026-08-15: trigger detail redesigned to the rule-page anatomy — page-head sentence with
the Enable switch opposite it, topbar ENABLED/DISABLED pill removed, Edit ghost + overflow stay
dynamic-only, Run now remains jobs-only. Detail-surface expectations moved to
`ET-web-trigger-detail-rule-page`; this scenario keeps catalog rendering, row → detail navigation,
breadcrumb, Create CTA, and the Loop create seed. Status reset to untested pending a walk.

QA 2026-08-15: an isolated Bruno walk passed the Triggers catalog search and
clear flow, row-to-detail navigation, breadcrumb/back/forward behavior, managed
trigger controls, and the Jobs canary. Run now appeared only on the Job detail.

QA impact 2026-08-20: catalog ListingToolbar search height now uses `--height-search`
(28px) to match RouteNav / Filter / view pills. Reset the listing chrome walk.

QA 2026-10-03: catalog search/Cards and Loop detail entry pass, but submitting a webhook from the live preview hides a server validation error. The existing silent-submit issue is reopened; repair and fresh replay are pending. Evidence: trigger-rule-bruno-ended.json and trigger-rule-retry-bruno-ended.json in this cycle.

QA 2026-10-03: hidden preview submission errors, rejected mapping examples and invalid sample JSON are repaired and re-walked. The full catalog verdict remains untested/Pending until its separately listed concurrency, refetch-failure and runtime-disable obligations are walked. Trigger detail closure lives in ET-web-trigger-detail-rule-page.

QA 2026-10-04: a retained scoped Loop run immediately reverses an explicit Global choice. The background-window bug is reopened for the route-adoption path. Catalog concurrency and runtime-off controls have current live evidence, but this scenario remains Pending for repair and the remaining charter legs.

QA 2026-10-04 final replay: Global survives retained scoped Loop windows, a failed global Job
and reload. Both catalogs retain rows with explicit offline waiting state and resume continuation.
Real service failures preserve cached definitions and show runtime/history errors; ordinary reload
restores controls. Two different Loop seeds are consumed once in both editors, cancelled drafts
do not return, and unseeded creation retains its default target. Route/project changes clear old
editor data; a pending Job does not disable another Job detail. Prior unchanged CRUD, managed
source, row/card and concurrency evidence is reused from this cycle. The scenario is Fixed;
the new repair commit stamp follows its required gate.
