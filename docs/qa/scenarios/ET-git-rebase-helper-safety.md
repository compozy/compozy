---
id: ET-git-rebase-helper-safety
area: ET
title: Diagnose and validate a rebase without corrupting file paths
persona: Ada
journey: J-offer-runnable-capabilities
expected: The git-rebase helper reports conflicted paths intact, reaches zero cleanly after resolution, and recommends the repository-owned gate from the repository root
entry_points: bash .agents/skills/git-rebase/scripts/analyze-conflicts.sh; bash .agents/skills/git-rebase/scripts/validate-merge.sh
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-coderabbit-git-rebase-helper-20260815-001221-669049-lab/qa-artifacts/qa/analyzer-conflict.txt; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-git-rebase-helper-20260815-001221-669049-lab/qa-artifacts/qa/git-conflict-read.txt; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-git-rebase-helper-20260815-001221-669049-lab/qa-artifacts/qa/analyzer-resolved.txt; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-git-rebase-helper-20260815-001221-669049-lab/qa-artifacts/qa/validator-staged-marker.txt; /Users/pedronauck/dev/qa-labs/compozy-coderabbit-git-rebase-helper-20260815-001221-669049-lab/qa-artifacts/qa/validator-resolved.txt
last_report: docs/qa/reports/2026-08-14-coderabbit-git-rebase-helper.md
overlaps:
---

This scenario owns the public shell-helper behavior changed by the CodeRabbit remediation. The walk uses a real temporary Git repository with a conflicted filename containing spaces, then resolves the conflict and reads the state again through Git and both bundled helpers.

Walked on 2026-08-14 in an isolated CLI-only lab. The analyzer preserved the spaced filename as one path and reported one unresolved conflict before resolution and zero afterward. The validator rejected staged conflict markers with exit 1, passed after they were removed, reported the staged path once, and recommended only repository-root gate and Turbo commands.
