---
id: RT-compozy-environment-namespace
area: RT
title: Use only the Compozy process environment namespace
persona: Dora
journey: J-validate-compozy-hard-cut
expected: Runtime home, managed-session, hosted-MCP, Web proxy/assets, QA/build, and provider-policy environment contracts use their COMPOZY_* names; retired variables are never read as fallbacks, while provider credentials and permitted pass-through variables still reach only their intended process.
entry_points: COMPOZY_HOME; COMPOZY_MANAGED; COMPOZY_MCP_SERVE_TOKEN; COMPOZY_WEB_API_PROXY_TARGET; Web asset/distribution variables; provider env-policy and process allowlists; hosted-MCP env injection
qa_status: skipped
bug_ids: BUG-20260727-runtime-legacy-identity
fix_status: fixed
retest_status:
fix_commits: e4df8634
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/bootstrap.env; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/api-status.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-integration-rerun.log
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: RT-compozy-home-isolation; ET-compozy-native-tool-invocation
---

story: As the runtime administrator, I can reason about one environment namespace and know that a
retired variable cannot silently redirect runtime state, credentials, or agent capabilities.

QA impact 2026-07-27: Task 12 derived this missing cross-surface row from the Task-02 environment
hard cut. Planning only; Task 13 owns the isolated runtime evidence.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
