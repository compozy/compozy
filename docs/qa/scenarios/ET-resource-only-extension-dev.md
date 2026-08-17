---
id: ET-resource-only-extension-dev
area: ET
title: Iterate on a resource-only extension without a build toolchain
persona: Bruno
journey: J-extension-dev-lifecycle
expected: A native extension that declares only static agents, skills, loops, automation, or layouts builds without package.json or go.mod and without running build or describe commands; dev and reload publish its resources only to the selected workspace, identical input keeps the same generation, changed valid input creates a new generation, and an invalid edit leaves the last-good generation active.
entry_points: `compozy extension build <dir>`; `compozy extension dev <dir> --workspace <ref>`; `compozy extension reload <name> <dir> --workspace <ref>`; `compozy extension dev <dir> --watch`; `GET /api/agents?workspace=<ref>`
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /home/franciscpd/dev/qa-labs/compozy-resource-only-extension-dev-20260817-020712-286410-lab/qa-artifacts/qa/journey-log.jsonl; /home/franciscpd/dev/qa-labs/compozy-resource-only-extension-dev-20260817-020712-286410-lab/qa-artifacts/qa/qa-audit-report.json; /home/franciscpd/dev/qa-labs/compozy-resource-only-extension-dev-20260817-020712-286410-lab/qa-artifacts/qa/teardown.json; /home/franciscpd/dev/qa-labs/compozy-resource-only-extension-dev-20260817-020712-286410-lab/project/resource-only-kit/dist/gen-6542fc8f1f0d926e9c2eff47fe0e6f4040f96a0f38b44499bad1c1cc681c3eb5
last_report: docs/qa/reports/2026-08-17-resource-only-extension-dev.md
overlaps: ET-extension-dev-reload-loop; ET-agent-plugin-dev-reload
---

Issue #421 adds the passive resource-kit lane to the existing immutable generation and workspace
overlay lifecycle. This scenario owns absence of a language toolchain and absence of extension
build/describe subprocesses; the generic dev scenario remains canonical for logs, executable tool
invocation, and published-instance restoration.

The failure probes are part of the user promise: an authored subprocess, capability, permission,
dynamic publication family, or explicit build command must fail with a toolchain diagnostic instead
of being silently treated as a passive kit. Invalid static resource edits must fail before the daemon
replaces the workspace's active generation.

QA 2026-08-17: Bruno built a passive agent kit twice to the same generation, linked it to one
workspace, observed it only through that workspace's public agent catalog, reloaded a changed prompt,
and confirmed malformed YAML left the new last-good generation active. Removal cleared the projected
agent, and a code-backed scaffold remained callable as the adjacent compatibility canary. The
source-freeze retest also confirmed the workspace agent through `compozy__workspace_describe` before
strict evidence audit and clean teardown.
