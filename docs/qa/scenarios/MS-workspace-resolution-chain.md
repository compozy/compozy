---
id: MS-workspace-resolution-chain
area: MS
title: Resolve workspace context through one precedence chain
persona: Ada
journey: J-operate-workspace-context
expected: Workspace-scoped CLI commands resolve positional ref, flag, environment, validated session identity, then nearest enclosing cwd in that order without registering a nested directory.
entry_points: compozy workspace info; compozy loop run; compozy session new; compozy config set --scope workspace; compozy memory list
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-workspace-resolution-provenance; ET-native-workspace-scope-isolation; RT-session-cwd-resume
---

Graduated from seed `cfg-07-workspace-resolver-modes`.

In one isolated `COMPOZY_HOME`, register `ws-alpha` at `/tmp/alpha`, a nested workspace at
`/tmp/alpha/packages/nested`, and `ws-beta` at `/tmp/beta`. From a subdirectory of each root, run
workspace info, Loop, config, memory, and session creation without `--workspace`; confirm the nearest
registered root wins and the catalog does not gain a subdirectory registration.

Repeat with `COMPOZY_WORKSPACE=ws-beta`, an explicit `--workspace`, and a positional workspace ref.
Confirm precedence is positional over flag, flag over environment, environment over validated
session identity, and identity over cwd. Partial, stale, global, or workspace-less credentials must
fall through. From an unregistered directory, expect the shared parseable error and registration
guidance; only `session new` may auto-register that genuinely new directory.

QA impact 2026-07-28: workspace resolution changed across CLI, HTTP/UDS lookup, and session creation.
Planning flag only; no QA replay ran in this implementation slice.
