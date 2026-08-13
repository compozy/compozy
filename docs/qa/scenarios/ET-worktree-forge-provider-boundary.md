---
id: ET-worktree-forge-provider-boundary
area: ET
title: Serve assisted exit through the GitHub forge provider without leaking credentials
persona: Dora
journey: J-worktree-management
expected: The selected forge.provider reports its vocabulary and capabilities, the bundled GitHub extension resolves credentials in binding then gh then absent order, status and PR creation stay idempotent, provider failures degrade truthfully, and no token reaches logs, payloads, events, SSE, memory, or the daemon process.
entry_points: forge.provider forge/capabilities|forge/status|forge/pr_create; extensions/forge/github; GITHUB_TOKEN secret binding; gh auth token fallback; compozy worktree status --forge|pr -o json; HTTP/UDS exit routes
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/cli-worktree-commit-forge.jsonl; internal/daemon/daemon_worktree_e2e_integration_test.go
last_report: docs/qa/reports/2026-08-13-worktree-support.md
overlaps: RT-worktree-exit-pr-idempotency; RT-worktree-exit-browser-fallback
---

QA impact: Task 05 adds the forge provider contract and bundled GitHub implementation. The walk must
exercise provider selection and the safe causes `rate_limited`, `credential_expired`,
`unsupported_remote`, and `credential_absent`, then disable the provider and prove local exit plus
the sanitized browser tier remain available.
