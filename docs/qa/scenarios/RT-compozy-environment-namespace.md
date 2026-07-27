---
id: RT-compozy-environment-namespace
area: RT
title: Use only the Compozy process environment namespace
persona: Dora
journey: J-validate-compozy-hard-cut
expected: Runtime home, managed-session, hosted-MCP, Web proxy/assets, QA/build, provider-policy, and bridge-template environment contracts use their COMPOZY_* names; equivalent AGH_* variables are never read as fallbacks, while provider credentials and permitted pass-through variables still reach only their intended process.
entry_points: COMPOZY_HOME; COMPOZY_MANAGED; COMPOZY_MCP_SERVE_TOKEN; COMPOZY_WEB_API_PROXY_TARGET; Web asset/distribution variables; provider env-policy and process allowlists; bridge manifests/templates; hosted-MCP env injection
qa_status: pass
bug_ids: BUG-20260727-runtime-legacy-identity
fix_status: fixed
retest_status: pass
fix_commits: e4df8634
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/bootstrap.env; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/api-status.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-integration-rerun.log
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps: RT-compozy-home-isolation; ET-compozy-native-tool-invocation
---

story: As the runtime administrator, I can reason about one environment namespace and know that a
retired variable cannot silently redirect runtime state, credentials, or agent capabilities.

QA impact 2026-07-27: Task 12 derived this missing cross-surface row from the Task-02 environment
hard cut. Planning only; Task 13 owns the isolated runtime evidence.
