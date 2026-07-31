---
id: ET-api-mcp-catalog-install
area: ET
title: Install a curated MCP server through the shared API
persona: Ada
journey: J-agent-marketplace-parity
expected: HTTP and UDS expose the same manifest-v2 MCP install contract, accept only declared typed `values.inputs[id]` values or Vault refs, apply the entry default scope when absent, reject client overrides of locked launch/auth fields, serialize concurrent Vault and sidecar mutations, report the persisted config-apply record and active generation, compensate newly created Vault refs after config-write failure, garbage-collect only superseded install-owned refs, return a committed warning when rollback cannot fully restore prior state, and return configured input names without binding refs or secret values. A post-commit install-event failure returns `mcp_install_event_persist_failed` without hiding the committed server.
entry_points: POST /api/settings/mcp-servers/install over HTTP; POST /api/settings/mcp-servers/install over UDS; GET /api/settings/mcp-servers
qa_status: skipped
bug_ids: BUG-20260715-mcp-install-null-values
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/notes/mcp-guided-oauth-workspace-isolation.json; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/023-mcp-catalog-install;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: ET-api-marketplace-namespace; ET-cli-mcp-install; MS-029
---

Skipped in the 2026-07-30 MCP 2026/catalog-v2 closeout: no HTTP/UDS install parity observation was retained.

Historical QA note: required-nullable values presence and config-apply response coverage remains pending.

Task 10 planning note: the MS-011 overlap was a mis-link (memory health); the settings-CRUD neighbor is MS-029.

Added by marketplace Task 03. QA should compare HTTP, UDS, and CLI payloads against one persisted server, verify the non-loopback privileged HTTP guard, exercise missing and shared Vault refs, run concurrent installs against one scope without losing definitions or secrets, and confirm `apply` truth remains separate from MCP probe/readiness when the target server is unreachable.

QA impact 2026-07-16: direct HTTP/UDS requests must include `values`; explicit `null` remains valid
for input-free entries, while omission returns `400` without calling the install owner.

QA impact 2026-07-16: replacing a typed secret with a present shared ref deletes the superseded
canonical ref but retains the shared ref. A forced Vault deletion failure restores the prior binding
and owned secret.

QA impact 2026-07-17: settings/install responses hard-cut secret bindings. PUT preservation uses
`preserve_secrets` against the exact target while responses expose presence metadata only.

QA impact 2026-07-17: plain env reads now expose `env_keys`; exact-target PUT preservation uses
`preserve_env`. Force partial secret restoration and definition-restoration failure separately and
assert the committed definition plus warning. Force install-event persistence failure and assert the
full committed response plus `mcp_install_event_persist_failed` over HTTP and UDS.

QA result 2026-07-29: explicit-null, omitted-values, locked-field, required-field, Vault-ref,
provenance, apply-truth, redaction, replacement, and cleanup branches passed over real HTTP, UDS,
and CLI surfaces. The historical null-values fix passed its fresh retest. Fault-injected config,
rollback, restoration, and event-persistence branches remain Pending because no public deterministic
fault owner exists in the current QA envelope.
