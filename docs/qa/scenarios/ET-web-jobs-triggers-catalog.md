---
id: ET-web-jobs-triggers-catalog
area: ET
title: Jobs and Triggers catalog plus deep-linkable detail
persona: Bruno
journey: J-24
expected: `/jobs` and `/triggers` render as ListingPage catalogs (PageHead + ListingToolbar search/filters/view + rows/cards) instead of SplitPane master-detail; row click opens `/jobs/$jobId` or `/triggers/$triggerId` with breadcrumb parent link; Create CTA stays in topbar actions; `?create=loop&loop=` from Loop detail still opens the create editor seeded at that Loop; dynamic Edit/Delete/Run now live in topbar actions on detail.
entry_points: web `/jobs`; web `/triggers`; Loop detail Add schedule/trigger CTAs
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: TA-052; TA-056; TA-automation-crud-loop-target; LP-033
---

Added by Route Chrome + catalog migration (2026-07-17). Selection moved from local state to detail child routes.

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
