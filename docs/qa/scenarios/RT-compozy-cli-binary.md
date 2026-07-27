---
id: RT-compozy-cli-binary
area: RT
title: Run the Compozy CLI and shipped helper binaries
persona: Ada
journey: J-validate-compozy-hard-cut
expected: Source builds, local development helpers, code generation, catalog tooling, and the Daytona sidecar invoke only their Compozy binary names; compozy version and status work and no legacy `agh` command alias is installed.
entry_points: go list -m; make build; ./bin/compozy version; compozy status -o json; compozy-codegen; compozy-catalog; compozy-daytona-sidecar
qa_status: pass
bug_ids: BUG-20260727-dirty-build-release-track
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/daemon-start.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/api-status.json; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-integration-rerun.log
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps: RT-compozy-home-layout
---

QA impact 2026-07-26: the CLI and every shipped helper command received a hard-cut
Compozy name with no compatibility alias. Planning flag only; the next QA cycle owns
the source-build and installed-command smoke.
