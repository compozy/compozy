---
id: ET-compozy-extension-contract-identity
area: ET
title: Load only Compozy extension, package, and OpenAPI identities
persona: Ada
journey: J-validate-compozy-hard-cut
expected: Extension loaders and real manifests accept only min_compozy_version, workspace packages use @compozy/*, generated contracts are openapi/compozy.json plus web/src/generated/compozy-openapi.d.ts and compozy-daemon-openapi.d.ts, and SDK/scaffolder/site readers resolve those canonical names without a legacy alias or dual manifest field.
entry_points: extension.toml and JSON manifest loading; compozy extension list|inspect; package.json workspace manifests; openapi/compozy.json; openapi/compozy-daemon.json; web/src/generated/compozy-openapi.d.ts; web/src/generated/compozy-daemon-openapi.d.ts; SDK/scaffolder and site API-reference readers
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-compozy-official-skill-discovery; ET-compozy-public-brand-navigation
---

story: As an autonomous extension operator, I can load and inspect one canonical Compozy contract
family without guessing which package, manifest field, or generated OpenAPI declaration is active.

QA impact 2026-07-27: Task 12 derived this missing extensibility/contract row from the Task-02 and
Task-04 hard cuts. Planning only; Task 13 owns the local loader and generated-reader evidence.

