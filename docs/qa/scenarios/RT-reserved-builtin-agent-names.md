---
id: RT-reserved-builtin-agent-names
area: RT
title: Reject authored builtin agent identities
persona: Ada
journey: J-32
expected: Create, rename, duplicate, native-tool, and extension kit publication attempts for coordinator or dreaming-curator fail with agent_name_reserved, create no authored directory, and leave the agent catalog unchanged.
entry_points: compozy agent create|update|duplicate; POST/PUT /api/agents over HTTP or UDS; compozy__agent_create; extension enable; pre-existing $COMPOZY_HOME/agents/<reserved-name>/ directory at boot (discovery skip); docs configuration/agent-md reserved builtin names
qa_status: untested
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence:
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: RT-081;MS-inspect-background-role-routing
---

QA impact 2026-07-23: builtin identities are newly reserved across every authoring surface. Planning flag only; the next QA cycle owns the real-user rejection sweep.

Planning 2026-07-24 (Task 05): entry points widened to the discovery-skip surface (a pre-existing on-disk reserved directory must be skipped at boot with a warning diagnostic naming the path, and must never shadow the builtin) and the agent-md docs note real users will read first. Session charter: CH-reserved-builtin-name-sweep.

The create/update/duplicate/native/normalization/near-miss sweep and a pre-existing on-disk coordinator shadow also passed; the shadow was diagnosed and skipped while the virtual builtin continued to resolve, and neither builtin appeared in CLI, HTTP, UDS, native, or fleet catalogs.

QA impact 2026-08-02: packaged agents now publish only through extension enable. Reset to verify the
same reserved-name invariant through that surviving lifecycle.
