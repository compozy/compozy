# BUG-20261003-automation-global-detail-refused: Global catalog entries refuse to open

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-24, open a project-owned automation from the Global catalog
- **Scenarios:** TA-052; TA-056
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Bruno finds a project job or trigger in the Global catalog, but opening it reports that it
belongs to another project. The same profile can read the definition through the public API.
The detail panel provides no route to inspect or manage the selected definition in Global.

## Reproduction

- **Charter:** CH-038 · **Tour:** Feature Tour
- **Environment:** desktop 1512×862, en-US, wifi-fast; browser-use, real daemon/Web on 55651.
- **Build:** v0.3.0-beta.29-30-g664f24775-dirty (cursor repair), committed source f38ee2ec1.

1. Select archive-editions in Global and open Triggers from the dock.
2. Filter Source to From package and open editorial-automations/Editorial pack review.
3. The detail panel says Unable to load details / This trigger belongs to another project.
4. Open Jobs, search November edition 01, and open that dynamic definition.
5. The job detail similarly says This job belongs to another project.
6. Read each package definition by ID through HTTP with profile=archive-editions: both return 200.
7. Enter Recovery Editorial and open the same package trigger: its detail renders.

**Expected:** the Global catalog can inspect its returned project-owned definitions. Selecting a
specific project still refuses a different project's detail.
**Actual:** the Web rejects all project-owned definitions whenever the active workspace is null.

## Evidence

All artifacts are in docs/qa/evidence/2026-10-02-untested/:

- automation-source-trigger-package-open.json/.png: package trigger refusal in Global.
- automation-global-job-project-refusal.json/.png: dynamic job refusal in Global.
- automation-source-{jobs,triggers}-package-direct.json: public reads return the owned definitions.
- automation-project-package-trigger-readonly.json: the same trigger renders in its project.
- automation-crud-bruno-ended.json: stopped 130-frame recording, complete bounded debrief.

## Fix

- **Root cause:** automationMatchesActiveWorkspace recognizes a global definition or an exact
  concrete workspace match, but treats the Global lens (null workspace) as a mismatch. Both detail
  view models suppress the server-returned item and its run history. Editors also need the loaded
  definition's workspace for their target catalog when the containing view is Global.
- **Fix commit:** pending
- **Regression test:** existing use-automation-trigger-detail-page.test.tsx; Global projection and
  concrete foreign-project refusal. The original Chrome job/trigger legs must be replayed.

## Verification

Fresh Bruno replay opens both project-owned package details in Global. Dynamic project jobs and
triggers keep the workspace-only editorial-reviewer target while editing name, prompt and job
schedule; independent CLI/UDS reads and reload confirm the saved definitions. Managed package
and config controls omit authored-field editing/deletion, and enabled overlays persist. Every
definition is disabled again at the end, with zero agent sessions started.

The existing detail-hook suite fails before the repair for the Global projection and passes
afterward while retaining concrete foreign-project refusal. Final-source targeted checks pass
20 tests across three existing suites; root Turbo build, lint and typecheck pass. The attempted
fourth test selector matched no file; it adds no coverage claim. The required make gate passes
every selected lane, including all 693 Web files / 6,908 tests; the target structural audit passes.
React Doctor reports the same two pre-existing complexity warnings: baseline 91, repaired 93;
cyclomatic complexity decreases by one in each hook without suppressions or config changes.

Evidence: automation-global-detail-bruno-ended.json (92-frame stopped recording),
automation-global-detail-{job-saved-reload,package-trigger-green}.png,
automation-global-details-{red,access-tests,access-web}.json and
automation-global-details-doctor-comparison.json. The original managed CLI refusal evidence
remains in automation-managed-boundaries-proof.json.
Delivery receipts: automation-global-details-{delivery-gate,gate-status,audit}.json.

The later project reload changes archive-editions to its independently remembered default profile.
The official profile contract intentionally preserves a viewing profile across a breadth change
without rewriting either remembered slot. No explicit project-profile choice was made in that leg;
its not-found response is not filed as a profile regression. The next replay must explicitly select
archive-editions for the project before asserting reload persistence.

That explicit project selection now passes reload. A separate observation on returning from
Global to a remembered project after refresh remains under investigation; it does not recur on
the original Global detail replay and is not evidence against this detail-access repair.

The suspected Source-menu mouse failure was a driver artifact: click_at_xy emits press/release
without moving the pointer. Fresh normal pointer movement opens both menus; scoped CLI reads
and reload confirm the config filters. No filter production code changed. Evidence:
automation-pointer-bruno-ended.json and automation-pointer-triggers-config-filter.png.
