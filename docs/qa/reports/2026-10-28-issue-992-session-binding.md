# QA Run Report — 2026-10-28 — Existing session worktree binding

- **Scope:** Issue #992: bind, change and unbind an existing ordinary session without replacing its identity or conversation.
- **Cadence tier:** targeted
- **Build:** Implementation rebased onto main at 7ccc00670; provider and refusal re-walks completed; current-head CI tracked in PR #1002.
- **Status:** targeted runtime pass; delivery gates tracked in PR #1002

## Session Matrix & Results

| Charter | Journey / Scenario | Persona | Tour | Status |
|---|---|---|---|---|
| CH-session-environment-continuity | RT-session-worktree-lifecycle: Claude cwd, sentinel, continuity | Ada | Data | Pass |
| CH-session-environment-continuity | RT-session-worktree-lifecycle: Codex cwd, sentinel, continuity | Ada | Data | Pass |
| CH-worktree-lifecycle-surface-parity | Bind/change/unbind through CLI, HTTP, UDS and native tools | Bruno | Feature | Pass |
| CH-session-environment-continuity | Restart persistence and refusal boundaries | Théo | Data | Pass (focused) |
| CH-worktree-lifecycle-surface-parity | Existing-session Web environment control and unchanged fork | Bruno | Feature | Pre-sync pass; current-head CI on PR #1002 |

## Preliminary Engineering Evidence

- The session integration suite passed with race detection, including binding changes, released process replacement, delivery exclusion, stopped-session recovery, and metadata preservation on a refused catalog CAS.
- The first browser run exposed a missing environment-notice augmentation parser in the ACP fixture. The canonical parser was updated and its owning race-enabled test passed. This was fixture-contract drift; the browser continuity assertions remain unchanged.
- Bun 1.4.4 produced unrelated embedded-linter generator drift. Using the repository-pinned Bun 1.4.0 passed codegen-check. The local full gate was interrupted on the operator's instruction to reserve the full matrix for CI. Completed lanes passed codegen and Go lint; focused checks passed after correcting route expectations and the intentional current-CWD precedence contract. macOS hook checks passed with a canonical `TMPDIR=/private/tmp`. No local full-gate pass is claimed.
- These observations are preliminary and do not establish final-head delivery or live-provider completion.

## Session Debriefs

Pre-sync exploratory run: Claude Sonnet 5 and Codex GPT-6 Astra each completed root → checkout A → checkout B → root in the same session. Every turn reported the expected `pwd` and recalled its original phrase. Independent filesystem reads confirmed distinct `ROOT_*`, `CHECKOUT_A_*`, and `CHECKOUT_B_*` sentinel contents in the three locations. CLI bind, HTTP change, native-tool unbind, and direct UDS unbind returned the same session identities. These results must be repeated against the synchronized implementation.

The browser run passed all three selected scenarios: native-load binding, bounded-replay binding, and the existing worktree fork. The catalog stream suite passed 24 tests, including external CWD changes refreshing only the owning session's file search. Browser automation uses real daemon/Git with ACP fixture processes; it is separate from the provider evidence.

The inherited containment charter predates in-place binding and expressly forbids it. The new `CH-session-environment-continuity` owns this expanded mission without rewriting the historical charter.

## Synchronized Focused Verification

- Session binding lifecycle integration, API/native adapters, guarded catalog CAS, schema compatibility, and HTTP/UDS route contracts passed.
- Both affected Web suites passed (environment control and catalog streams); Web typecheck passed through root Turborepo. Main's new font dependency initially blocked resolution; installing the unchanged frozen lockfile fixed the environment.
- Codegen completed after rebase and regenerated the Go SDK type partitions.
- CI owns the full matrix per the operator's instruction; no further local full gate, full race lane, or E2E suite is run.

## Synchronized Live-Provider Evidence

On the implementation rebased onto main, the same two existing sessions completed root → A → B → root sequentially (Claude Sonnet 5, then Codex GPT-6 Astra). Both recalled their original phrase on every checkout, read the expected checkout sentinel, and wrote independently verified final sentinels. Both invoked the native terminal without a cwd override and observed the selected checkout/root.

Claude continued at B after a graceful daemon restart between binding and prompting. Codex continued at B after SIGKILL of the registered, identity-verified lab daemon at that same boundary and subsequent restart. Only this isolated lab was stopped. The catalog still contained the two original session IDs.

CLI, HTTP, direct UDS and native mutation receipts are retained privately. The public file-search endpoint switched root-only → checkout-only → root-only filenames immediately, with no extra provider turn. A concurrent mutation during a confirmed active Codex prompt returned `409 session_busy`; stale expectation returned `409 session_worktree_conflict`; an unknown checkout returned `404 worktree_not_found` over HTTP.

The native unknown-checkout walk exposed an incorrect generic `tool_not_found` translation. The adapter now preserves worktree refusal codes, with an extension to its canonical suite and regenerated wire contracts. The live re-walk returned `worktree_not_found` from the native tool. An archived-session attempt returned `409 session_not_promptable`, and unarchiving restored availability. This targeted continuity run does not claim the bootstrap's broader multi-agent/task orchestration audit.

The isolated lab teardown completed with `TEARDOWN_ALL_CLEAN=true`; only registered/identity-verified lab processes were stopped. Private evidence remains retained.

## Review and CI Regression Follow-up

- The session integration suite reproduced caller cancellation stranding a pending journal in both active and stopped paths. A detached, bounded lifecycle context now completes admitted transitions; both cases pass and the next prompt proceeds.
- The stopped-path notification test reproduced a subscriber being refused while the conversation lock remained held. Publication now uses the same post-release path as active sessions; the regression passes.
- Catalog stream tests cover both a changed worktree ID and a changed workspace root, while proving an ordinary upsert does not restart file search. The two affected Web suites passed 31 tests.
- API tests assert distinct busy/stale error codes despite their shared HTTP 409 status. The CI native inventory failure was fixed by adding presentation entries for both new tools, preserving the exact inventory check.

## Final Status

Targeted runtime and focused checks passed. Current-head CI, browser artifacts and reviewer dispositions are recorded in [PR #1002](https://github.com/compozy/compozy/pull/1002). This report does not substitute for those delivery gates.
