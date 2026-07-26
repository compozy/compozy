---
id: RT-compozy-cli-binary
area: RT
title: Run the Compozy CLI and shipped helper binaries
persona: Ada
journey: J-operate-daemon-schema
expected: Source builds, local development helpers, code generation, catalog tooling, and the Daytona sidecar invoke only their Compozy binary names; compozy version and status work and no agh command alias is installed.
entry_points: make build; ./bin/compozy version; compozy status -o json; compozy-codegen; compozy-catalog; compozy-daytona-sidecar
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-compozy-home-layout
---

QA impact 2026-07-26: the CLI and every shipped helper command received a hard-cut
Compozy name with no compatibility alias. Planning flag only; the next QA cycle owns
the source-build and installed-command smoke.
