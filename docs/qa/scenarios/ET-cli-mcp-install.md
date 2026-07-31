---
id: ET-cli-mcp-install
area: ET
title: Install a curated MCP server through the CLI
persona: Ada
journey: J-agent-marketplace-parity
expected: `compozy mcp install` validates manifest-v2 input ids and the final server name before reading stdin, accepts `--set id=value`, `--secret id`, and `--vault-ref id=vault:...` only for declared typed inputs, applies the catalog default scope when omitted, persists scope-qualified Vault refs, and returns JSON without secret values or binding refs. Human and TOON output remain reduced summaries. A post-commit event failure is visible as `mcp_install_event_persist_failed` with the committed server intact.
entry_points: compozy mcp install <entry> --set id=value -o json; compozy mcp install <entry> --secret id -o json; compozy mcp install <entry> --vault-ref id=vault:mcp/shared/ref -o json; compozy mcp install <entry> --scope workspace --workspace <id> -o json
qa_status: pass
bug_ids: BUG-20260729-mcp-cli-json-parity
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/notes/cli-mcp-install-inline.json; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/notes/cli-mcp-install-vault.json; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/notes/cli-mcp-install-foreign.stderr; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/notes/mcp-list-after-cli-installs.json
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: ET-api-mcp-catalog-install; ET-cli-marketplace-search; MS-029
---

Passed in the 2026-07-30 final rerun: Brave Search installed through both an inline secret and an
existing Vault ref, the resulting reads stayed redacted, and a foreign-workspace Vault ref was
rejected before mutation. The synthetic post-commit event-persistence fault remains
`blocked-decision` because no public healthy-daemon owner can create that internal failure state.

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

QA impact 2026-07-30 deep-review remediation: reset after install validation moved before secret
prompting and Vault-ref authorization was hardened. Verify a workspace install accepts only its own
`vault:mcp/ws/<workspace>/<server>/...` bindings or an explicit `vault:mcp/shared/...` binding, and
rejects another workspace's ref before mutation or secret input. The public event-persistence fault
branch may remain `blocked-decision` only if no legitimate fault owner exists.
