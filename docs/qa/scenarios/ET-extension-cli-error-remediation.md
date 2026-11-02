---
id: ET-extension-cli-error-remediation
area: ET
title: Recover from actionable extension CLI failures
persona: Ada
journey: J-extension-distribution
expected: Policy-blocked install, daemon-unavailable, missing-Git, and outdated-Git failures print human remediation and a `try:` command identical to the diagnostic carried by JSON output.
entry_points: compozy extension install; compozy status; -o json
qa_status: pass
bug_ids: BUG-20260804-native-extension-remediation
fix_status: fixed
retest_status: pass
fix_commits: working-tree
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json;/Users/pedronauck/dev/qa-labs/compozy-go-modernization-closeout-20260804-121411-946266-lab/qa-artifacts/qa/evidence/extensions-closeout.json
last_report: docs/qa/reports/2026-08-04-go-modernization-closeout.md
overlaps: ET-cli-extension-sideload-policy-block; ET-022
---

Added by ext-improvs Task 07 for the shared root execution-error renderer. Walk both failure paths
with the stamped binary and compare the authored title, message, and suggested command across output
modes.

QA impact 2026-08-03: Git older than 2.37 now reports
`extension_git_version_unsupported` with `git --version` and an upgrade action across human, JSON,
HTTP/UDS, and native-tool output. Reset to untested for parity against the stamped binary.

QA 2026-08-04: passed after one QA-discovered parity fix. Missing and outdated Git now preserve the
same authored cause and recovery across human CLI, JSON, HTTP, and `compozy__extensions_install`;
untrusted backend detail remains masked. The native-tool regression was retested through the real
daemon after the fix.

Recovery coverage (2026-11-01): keep an unrelated extension with a failing provider registered.
Install a healthy resource-only package, then attempt an invalid package and verify rollback
removes its registration, installation and copied directory. Seed an existing package with
no installation rows; an operator's `extension remove <name> --global` must remove it and allow
reinstallation. Workspace-scoped agents must not gain authority to remove that orphan.
Verification: see [targeted upgrade and recovery evidence](../reports/2026-11-01-superseded-extension-recovery.md).

During orphan recovery, seed an extension-owned Vault write interrupted before its binding row.
Verify removal collects it across the orphan's workspace namespaces, preserves secrets belonging
to another extension or an operator, and restores the before-image if removal fails. A delayed MCP
failure resolved before removal must not reappear after reinstalling the same bundle.

While package retirement holds its mutation lock, issue a generic Vault PUT or DELETE for that
extension's workspace ref. The request must wait until retirement/rollback completes, then retain
its newer value or deletion. Refs in other packages must not enter the orphan metadata result set.
