---
id: ET-extension-quickstart-verbatim
area: ET
title: Reach a working extension by following the published quickstart verbatim
persona: Lea
journey: J-extension-newcomer-first-success
expected: A newcomer on a release-stamped binary who types only the commands printed in the published quickstart ends with an installed, invocable extension, having spent at most four actions and ten concepts, with no trust prompt and no undocumented step.
entry_points: https://compozy.com/runtime/guides/build-your-first-extension; `compozy extension init <name> --template tool-provider-go`; `compozy extension dev <dir>`; `compozy tool invoke ext__<name>__search --workspace . --input '{...}'`
qa_status: blocked-verify
bug_ids: BUG-20260729-public-extension-sdks-unpublished
fix_status: pending
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/newcomer/quickstart.json
last_report: docs/qa/reports/2026-07-29-ext-improvs.md
overlaps: ET-extension-code-first-authoring; ET-extension-dev-reload-loop
---

Task 11 retyped the published Go path from a clean directory outside the repository using the
release-stamped binary. `extension init` succeeded, but `extension dev` could not resolve the
generated `github.com/compozy/compozy/sdk/go v0.3.0-beta.1` dependency because the nested module
tag is unpublished. The TypeScript SDK coordinate is also absent from npm. The path remains
blocked until both public SDK artifacts exist; no repository-local dependency override was used.

Scorecard: walk the published page and count. The page claims four actions (`daemon start`, `init`,
`dev`, `tool invoke`) and nine concepts (extension, template, tool, input schema, handler,
generation, dev link, tool ID, reload). Recount by walking, and record a bug if the real path needs a
step the page does not print.

Verbatim discipline: retype the commands exactly as published, including `--workspace .` on
`tool invoke`, and do not substitute knowledge from other pages. E2E-007 replays the same fenced
block on a stamped binary, so a docs edit that breaks the path fails CI; this scenario covers what
CI cannot — whether a human reading the prose reaches the same place.

Also cover the TypeScript variant (`--template tool-provider-ts` plus `bun install`), the edit →
`reload` → observe loop, `logs --follow`, and `remove` restoring a clean workspace.

Dedup: ET-extension-code-first-authoring owns `init`/`build`/`validate` across all five templates and
ET-extension-dev-reload-loop owns the dev overlay lifecycle. This file owns only the published-path
fidelity and the concept and action counts.
