---
id: ET-extension-dx-scorecard
area: ET
title: Re-grade the extension developer experience against the binding scorecard
persona: Lea
journey: J-extension-newcomer-first-success
expected: A release-stamped newcomer path introduces at most ten concepts and reaches first success within four actions; own-source acquisition takes one command plus at most one consent; list, search, and Web reveal updates without a check command; the unchanged brief rubric re-grades SDK simplicity, completeness, and currency at A−/B/B or better.
entry_points: https://compozy.com/runtime/guides/build-your-first-extension; https://compozy.com/runtime/core/extensions/develop; https://compozy.com/runtime/core/extensions/install; https://compozy.com/runtime/core/extensions/manifest; https://compozy.com/runtime/core/extensions/permissions; `compozy extension init|dev|install|list|search`; /marketplace/extensions; sdk/go; sdk/typescript; sdk/examples
qa_status: blocked-verify
bug_ids: BUG-20260729-public-extension-sdks-unpublished
fix_status: pending
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/newcomer/quickstart.json
last_report: docs/qa/reports/2026-07-29-ext-improvs.md
overlaps: ET-extension-quickstart-verbatim; ET-extension-published-source-installs; ET-extension-passive-update-discovery
---

Use the measurement rubric and denominators from `.compozy/tasks/ext-improvs/_brief.md`; do not
substitute subjective impressions or award credit for repository-only examples.

Task 11 measured nine introduced concepts and three documented extension commands after daemon
setup, within the binding targets. The grade cannot be awarded: the second extension command fails
in a clean external workspace because neither public SDK coordinate is currently consumable. This
is a release blocker, not an extra newcomer action or a locally repairable documentation defect.
