---
id: RT-reserved-builtin-agent-names
area: RT
title: Reject authored builtin agent identities
persona: Ada
journey: J-32
expected: Create, rename, duplicate, native-tool, and bundle materialization attempts for coordinator or dreaming-curator fail with agent_name_reserved, create no authored directory, and leave the agent catalog unchanged.
entry_points: agh agent create|update|duplicate; POST/PUT /api/agents over HTTP or UDS; agh__agent_create; bundle activation; pre-existing $AGH_HOME/agents/<reserved-name>/ directory at boot (discovery skip); docs runtime/core/configuration/agent-md reserved builtin names
qa_status: pass
bug_ids: BUG-20260724-bundle-agent-snapshot-loss;BUG-20260724-reserved-bundle-error-mapping
fix_status: fixed
retest_status: pass
fix_commits: c841d7e06428c28e4e1b4ba8c17bccb4a103eea1;a1c966c01b40ae37372e4431704703acd92e679a
evidence: /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/reserved-bundle-http-fixed-probe.json; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/reserved-bundle-uds-fixed-probe.json; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/native-bundle-capable-session-history.json
last_report: docs/qa/reports/2026-07-24-agent-roles.md
overlaps: RT-081;MS-inspect-background-role-routing
---

QA impact 2026-07-23: builtin identities are newly reserved across every authoring surface. Planning flag only; the next QA cycle owns the real-user rejection sweep.

Planning 2026-07-24 (Task 05): entry points widened to the discovery-skip surface (a pre-existing on-disk reserved directory must be skipped at boot with a warning diagnostic naming the path, and must never shadow the builtin) and the agent-md docs note real users will read first. Session charter: CH-reserved-builtin-name-sweep.

QA 2026-07-24: bundle snapshot loss and reserved-error mapping were fixed and retested. CLI, HTTP, UDS, and a real `agh__bundles_activate` call reject the packaged coordinator with `agent_name_reserved`; HTTP/UDS return 422, the synthetic bundle path is named, and activation/agent catalogs remain unchanged.

The create/update/duplicate/native/normalization/near-miss sweep and a pre-existing on-disk coordinator shadow also passed; the shadow was diagnosed and skipped while the virtual builtin continued to resolve, and neither builtin appeared in CLI, HTTP, UDS, native, or fleet catalogs.
