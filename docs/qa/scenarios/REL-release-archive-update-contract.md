---
id: REL-release-archive-update-contract
area: REL
title: Reject archives the self-updater cannot consume
persona: Dora
journey: J-publish-compozy-beta
expected: Every Darwin and Linux archive passes the runtime-owned compressed-archive and extracted-binary policy after GoReleaser builds it and before the draft can be published; an incompatible archive stops publication with measured artifact details.
entry_points: .goreleaser.yml before_publish; go run github.com/magefile/mage@v1.17.2 updateArchiveCheck <archive>
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-359-auto-update-20260812-211235-947224-lab/qa-artifacts/qa/release-archive-contract-final.txt; /Users/pedronauck/dev/qa-labs/compozy-issue-359-auto-update-20260812-211235-947224-lab/qa-artifacts/qa/release-archive-oversized-rejection-final.txt
last_report: docs/qa/reports/2026-08-12-issue-359-auto-update.md
overlaps: REL-beta-self-update
---

Added for issue #359. The release producer and direct-binary updater consume the same Go policy;
Windows ZIP remains outside the supported in-place update platforms.

QA verdict 2026-08-12: the exact Mage hook accepted beta.13's 49,342,626-byte archive and
135,516,530-byte binary, then rejected a 269,484,032-byte binary with the measured 268,435,456-byte
policy limit before publication.
