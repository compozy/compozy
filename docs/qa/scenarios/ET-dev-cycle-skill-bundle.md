---
id: ET-dev-cycle-skill-bundle
area: ET
title: Use the bundled dev-cycle skills in a managed session
persona: Ada
journey: J-offer-runnable-capabilities
expected: A managed session lists and prompts with exactly the nine dev-cycle workflow skills, each view returns its bundled body, and a workspace-local override wins only in its owning workspace while another workspace keeps the global bundled declaration.
entry_points: compozy extension list; compozy skill list|view; compozy__skill_list|view; managed session prompt
qa_status: pass
bug_ids: BUG-20260727-runtime-legacy-identity
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-integration-rerun.log; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-e2e-web-final-2.log
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps: ET-003; ET-004; ET-skill-activation-gates
---

QA impact 2026-07-27: the bundled dev-cycle extension now publishes its nine workflow skills
globally to Compozy-managed sessions while preserving workspace-local lookup, cache, and prompt
isolation. Planning flag only; the next QA cycle owns execution.
