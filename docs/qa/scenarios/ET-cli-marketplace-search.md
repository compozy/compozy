---
id: ET-cli-marketplace-search
area: ET
title: Search the marketplace through structured CLI output
persona: Ada
journey: J-agent-marketplace-parity
expected: `compozy marketplace search` returns the canonical catalog envelope with truthful installed state, revision and pagination. JSON matches the daemon response; JSONL, human and TOON retain page metadata. Retired --kind flags and two-argument info fail before transport.
entry_points: compozy marketplace search [query] [--cursor <opaque>] -o json|jsonl|toon; compozy marketplace info <entry_id> [--source <name>]; compozy marketplace refresh
qa_status: untested
bug_ids: BUG-20260715-native-marketplace-extension-parity; BUG-20260715-marketplace-stale-report; BUG-20260729-marketplace-json-parity; BUG-20260729-marketplace-file-cursor-fence
fix_status: fixed
retest_status: ""
fix_commits: 8eeb8a38;351f3535
evidence: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-02-bundles-removal.md
overlaps: ET-007; ET-016
---

Current contract (2026-09-13): compare CLI canonical browse/detail with HTTP and UDS for the same source and installed-state scope. Continue without a kind selector, verify revision/cursor fencing and source identity, and verify old flags/arity cannot query or mutate. Final tasks09/10 own this walk. The historical grouped-kind evidence below does not validate the amended contract.

## Historical evidence

Historical QA note: opaque cursor continuation through structured CLI output remains pending.

Added by marketplace Task 02 after the hard cut to one discovery namespace. The next agent-surface QA cycle should compare CLI JSON byte semantics with HTTP and UDS for the same daemon state, including one isolated kind failure.

QA impact 2026-07-18: `--cursor` now continues a single `--kind` from `next_cursor`; grouped search
rejects it. Compare both pages with HTTP/UDS and prove the cursor remains bound to scope/workspace.

QA impact 2026-07-18: grouped JSON omits continuation metadata, while a single-kind continuation
rejects a cursor when its catalog projection changes. Confirm the structured error tells the
operator to restart from the first page.

QA impact 2026-07-18: single-kind human and TOON output append a Page block, and JSONL appends one
`type: "page"` record after the rows. Verify every format exposes `next_cursor` and available total,
stale, and diagnostic metadata without changing the JSON envelope.

QA result 2026-07-29: CLI JSON added a transport-only resolution field, and unchanged local
catalog refetches invalidated continuation cursors. Both canonical regressions, staged root fixes,
and the rebuilt CLI/HTTP/UDS replay are green; the scenario remains failed until the fixes have
governed commits.

QA impact 2026-08-02: the marketplace kind set is now exactly MCP, Extension, and Skill. Reset to
prove all structured modes and deterministic rejection of the retired kind.
