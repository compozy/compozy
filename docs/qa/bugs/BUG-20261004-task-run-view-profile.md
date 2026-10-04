# BUG-20261004-task-run-view-profile: An owned run opens as Run not found

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-drain-scheduler, open admitted work from Tasks Dashboard
- **Scenarios:** TA-048
- **Found:** 2026-10-04 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Dora opens a visible active run in resume-editorial. Its run page eventually says Run not found,
including after one clean reload. An independent public read under that profile finds the same run.

## Reproduction

- **Charter:** CH-scheduler-drain-recovery · **Tour:** Interrupt Tour
- **Environment:** desktop, 1512 × 862, local Wi-Fi, en-US; isolated real daemon

1. Select resume-editorial and its Studio Operations workspace.
2. Open the active maintenance run from Tasks Dashboard.
3. Observe Run not found; reload once and observe the same result.
4. Read the same run with profile=resume-editorial over HTTP: 200.

**Expected:** The run page uses the selected profile for its reads and cache identity.
**Actual:** The Web request omits profile and receives 404 from the default profile.

## Evidence

Receipts in docs/qa/evidence/2026-10-02-untested/:
- scheduler-drain-dora-replay-worker-open.json and run-detail-stall.png
- scheduler-drain-dora-replay-run-view-refusal.json and run-view-retry-observed.json
- scheduler-drain-dora-replay-run-view-independent.json
- scheduler-drain-dora-replay-ended.json

The exact run is run-b94e9c694e47324f, owned by task-eefc27c9d6b126fc. No query-string injection
or synthetic UI state was used to bypass the failed page.

## Fix

- **Root cause:** run queries, adapters and route preloads omitted the profile already carried
  by task detail. The run page's related reads/stream had the same gap. Three OpenAPI registry
  groups also skipped profile parameters, although task actor resolution already enforced them.
  A cold entry also launched parallel loaders before recovering the remembered profile. The
  application beforeLoad now hydrates the workspace lens and fetches its selection first,
  preserving any explicit local view. Failure uses the existing route error/retry boundary.
- **Fix commit:** pending
- **Regression test:** the existing run-page hook suite reproduces the omitted scope and
  exercises a scoped-to-default-to-aggregate switch without cache leakage. Existing adapter and
  route suites cover the wire/preload boundaries; the canonical public-schema suite reproduces
  missing selectors. The scheduler is explicitly kept outside profile selectors.

## Verification

Dora's scheduler-projection-fixed replay opens the queued follow-up from Dashboard and then
the Inspect dialog. The mounted run reads all carry resume-editorial and succeed. Cancellation
through CLI appears in the open run page without reload. One clean reload renders the same run,
but its initial task, run-history and run-detail preloads still issue profile=default and receive
404 before the mounted selected-profile reads succeed. The bug remains open for that entry race.
Evidence: scheduler-projection-fixed-dora-{open-run,run-inspect-web,restore-web,ended}.json and
the inspected run-inspect.png/run-reloaded.png. The persona session ends before diagnosis.

The final original-persona replay returns to run-b94e9c694e47324f. Completed status, the full
result, timeline and Inspect dialog render under resume-editorial. A fresh UDS read agrees.
The captured clean reload issues task, task-run list and run-detail requests with that profile;
all three return 200, no captured request selects another profile and no HTTP error occurs.
Dora returns to Dashboard with the selection intact; independent UDS confirms Running and
zero active/queued runs. All three screenshots were inspected and the nine-frame recording
was stopped. Receipts: profile-cold-entry-dora-{open,reload,independent,ended}.json and
profile-cold-entry-dora-reload-network.json. The existing route suite passes all 49 cases;
delivery gate and fix-commit recording remain pending.
