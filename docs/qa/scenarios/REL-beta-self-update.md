---
id: REL-beta-self-update
area: REL
title: Keep self-update on the running beta line
persona: Dora
journey: J-evaluate-compozy-beta
expected: A v0.3 beta direct binary selects the newest non-draft v0.3 beta, verifies and extracts its official archive within the runtime artifact policy, replaces only the isolated executable, and reports the installed beta version.
entry_points: compozy update; compozy update --check -o json; GitHub releases API; managed install detection
qa_status: pass
bug_ids: BUG-20260812-successful-update-recommends-retry
fix_status: fixed
retest_status: pass
fix_commits: working-tree
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-359-auto-update-20260812-211235-947224-lab/qa-artifacts/qa/update-check-final.json; /Users/pedronauck/dev/qa-labs/compozy-issue-359-auto-update-20260812-211235-947224-lab/qa-artifacts/qa/update-apply-final.json; /Users/pedronauck/dev/qa-labs/compozy-issue-359-auto-update-20260812-211235-947224-lab/qa-artifacts/qa/candidate-final-version.txt
last_report: docs/qa/reports/2026-08-12-issue-359-auto-update.md
overlaps: REL-beta-install-paths
---

QA impact 2026-07-27: Compozy migration Task 10 made update selection channel-aware and changed
managed lifecycle guidance to the active beta distribution identities. Planning flag only; Task
10's post-publish single-cut runbook owns real release/API behavior. Task 13 must not select or
simulate this scenario.

QA impact 2026-08-12: the updater now enforces the runtime-owned archive policy shared with the
release producer. Reset for an isolated direct-binary walk from beta.8 to the real beta.13 archive,
including structured output and executable replacement.

QA verdict 2026-08-12: an isolated fixed-source beta.8 candidate selected the real beta.13 release,
verified its signed checksum catalog, extracted the 135,516,530-byte binary, replaced only its lab
executable, and reported beta.13. The final JSON cleared the completed update recommendation.
