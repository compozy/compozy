# BUG-20260724-memory-extractor-agent-tier: Extractor emits agent tier metadata outside agent scope

- **Status:** verified
- **Impact (user-side):** Functional
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-digest-sessions-into-memory, extractor harvest
- **Scenarios:** MS-011
- **Found:** 2026-07-24 · **Report:** docs/qa/reports/2026-07-24-agent-roles.md

## Summary

The default Memory extractor produced a durable global candidate with `agent_tier = "global"`. Agent tier is valid only for agent-scoped memories, so the controller rejected the candidate and moved it to the extractor DLQ.

## Reproduction

- **Charter:** CH-dream-pipeline-canary · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US, isolated `devtool-oss-launch` lab

1. Run a normal provider-backed session while the default memory extractor is enabled.
2. Let the extractor return a global or workspace JSONL candidate with a non-empty `agent_tier`.
3. Drain the extractor and inspect `agh memory extractor list-pending -o json`.

**Expected:** The extractor boundary returns a contract-valid candidate; `agent_tier` is empty outside `scope = "agent"`.
**Actual:** The controller rejects the candidate with `memory controller: candidate frontmatter: agent tier requires agent scope` and records a DLQ item.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/archive/20260724T112506.161087000Z-20260724T112506.009718000Z-257.jsonl.processing.json`
- `/Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/memory-extractor-status-after-source.json`

## Fix

- **Root cause:** The extractor prompt presented `agent_tier` as a universal field, and `candidateFromExtractedLine` preserved it for every scope even though Memory v2 permits it only for agent scope.
- **Fix:** The prompt now makes `agent_tier` conditional, while the untrusted-output adapter strips it from global and workspace candidates before controller submission.
- **Fix commit:** `b6f7408439a68b9e5225b1b086770b4e37347e58`
- **Regression test:** `internal/daemon/memory_runtime_test.go` — `TestCollectMemoryExtractorOutput` covers both workspace and global candidates carrying an inapplicable tier.

## Verification

- **Retested:** 2026-07-24 from a rebuilt and restarted isolated daemon.
- A fresh provider-backed session produced two workspace memories; drain returned `remaining = 0`, no new DLQ record appeared, and health returned `status = "ok"` with two workspace files.
- Evidence: `memory-pipeline-retest-{drain,extractor-status,list,health}.json` under the run's `qa-artifacts/qa` directory.
- Scoped `-race` gates passed 1,396 daemon tests and 32 memory-prompt tests; repository lint passed.
