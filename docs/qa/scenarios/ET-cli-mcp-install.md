---
id: ET-cli-mcp-install
area: ET
title: Install a curated MCP server through the CLI
persona: Ada
journey: J-agent-marketplace-parity
expected: `compozy mcp install` validates field keys and the final server name before reading stdin, reads typed values from stdin or a hidden prompt, persists only scope-qualified Vault refs, preserves catalog provenance, and returns the complete install response in JSON without secret values or binding refs. Human and TOON output remain reduced summaries. A post-commit event failure is visible as `mcp_install_event_persist_failed` with the committed server intact.
entry_points: compozy mcp install <entry> --scope global --set KEY -o json; compozy mcp install <entry> --scope workspace --workspace <id> --set KEY -o json; compozy mcp install <entry> --vault-ref KEY=vault:mcp/shared/ref -o json; compozy mcp install <entry> --oauth-client-secret -o json; compozy mcp install <entry> --oauth-client-secret-vault-ref vault:mcp/shared/oauth -o json
qa_status: blocked-decision
bug_ids: BUG-20260729-mcp-cli-json-parity
fix_status: pending
retest_status: blocked-decision
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-agent-parity-final.json; /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-under-minute.json; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/023-mcp-catalog-install
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-api-mcp-catalog-install; ET-cli-marketplace-search; MS-029
---

Added by marketplace Task 03. QA should cover typed and choose-existing secret modes, global and two-workspace identity, a required-field rejection with no side effects, OAuth `next_step=authorize`, presence-only reads, and absence of plaintext or binding refs in CLI output, events, and logs. Config sidecars retain refs as runtime configuration.

QA impact 2026-07-17: typed values now enter through stdin or a hidden terminal prompt; argv carries
field names only. All CLI output formats expose configured field names and OAuth-secret presence,
never Vault refs.

QA impact 2026-07-17: malformed `--set`, `--vault-ref`, and `--name` inputs must fail before stdin or
the secret prompt is read. Compare `-o json` with HTTP/UDS for the complete server/apply/next-step/
warnings response, including a committed install whose event persistence fails.

QA result 2026-07-29: stdin-only values, validation order, presence-only output, catalog provenance,
apply truth, scoped Vault ownership, replacement, and cleanup passed. Workspace JSON added the
CLI-only `resolution_source` field; the scenario remains failed while the required structural writer
TechSpec is pending.
