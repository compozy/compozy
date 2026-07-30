---
id: ET-compozy-extension-contract-identity
area: ET
title: Load only Compozy extension, package, and OpenAPI identities
persona: Ada
journey: J-validate-compozy-hard-cut
expected: Extension loaders and real manifests accept only min_compozy_version, workspace packages use @compozy/*, the sole generated contract is openapi/compozy.json plus web/src/generated/compozy-openapi.d.ts, and SDK/scaffolder/site readers resolve those canonical names without aliases or a second daemon contract.
entry_points: extension.toml and JSON manifest loading; compozy extension list|inspect; package.json workspace manifests; openapi/compozy.json; web/src/generated/compozy-openapi.d.ts; SDK/scaffolder and site API-reference readers
qa_status: blocked-verify
bug_ids: BUG-20260727-runtime-legacy-identity
fix_status: fixed
retest_status: pass
fix_commits: e4df8634
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-e2e-web-final-2.log; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/api-status.headers;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-compozy-official-skill-discovery; ET-compozy-public-brand-navigation
---

story: As an autonomous extension operator, I can load and inspect one canonical Compozy contract
family without guessing which package, manifest field, or generated OpenAPI declaration is active.

QA impact 2026-07-27: Task 12 derived this missing extensibility/contract row from the Task-02 and
Task-04 hard cuts. Planning only; Task 13 owns the local loader and generated-reader evidence.

QA impact 2026-07-28: the second daemon OpenAPI and generated TypeScript artifacts were deleted, and
agent identity headers now live only in the canonical contract. Historical evidence remains, but
the changed hard-cut surface is untested for the next cycle.
