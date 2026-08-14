---
id: ET-spec-cycle-legacy-skill-retired
area: ET
title: Avoid the retired spec-cycle skill installer and duplicate runtime skill
persona: Ada
journey: J-offer-runnable-capabilities
expected: Installing or re-enrolling the spec-cycle extension contributes no extension-owned `compozy` skill, exposes no external agent-CLI installation path, writes to no external CLI home, and leaves the official bundled `compozy` skill as the sole owner of that name.
entry_points: compozy extension list; compozy skill list|view; managed spec-cycle enrollment
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-integration-rerun.log; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-e2e-web-final-2.log
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps: ET-compozy-official-skill-discovery; ET-spec-cycle-skill-bundle
---

QA impact 2026-07-27: the legacy spec-cycle `compozy` skill and external-CLI install behavior are
retired instead of retained as aliases or compatibility paths. Planning flag only; the next QA
cycle owns execution.
