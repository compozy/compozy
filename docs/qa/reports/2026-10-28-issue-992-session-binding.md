# QA Run Report — 2026-10-28 — Existing session worktree binding

- **Scope:** Issue #992: bind, change and unbind an existing ordinary session without replacing its identity or conversation.
- **Cadence tier:** targeted
- **Build:** Implementation rebased onto main at 7ccc00670; final provider and CI verification in progress.
- **Status:** in-progress

## Session Matrix & Results

| Charter | Journey / Scenario | Persona | Tour | Status |
|---|---|---|---|---|
| CH-session-environment-continuity | RT-session-worktree-lifecycle: Claude cwd, sentinel, continuity | Ada | Data | Pending |
| CH-session-environment-continuity | RT-session-worktree-lifecycle: Codex cwd, sentinel, continuity | Ada | Data | Pending |
| CH-worktree-lifecycle-surface-parity | Bind/change/unbind through CLI, HTTP, UDS and native tools | Bruno | Feature | Pending |
| CH-session-environment-continuity | Restart persistence and refusal boundaries | Théo | Data | Pending |
| CH-worktree-lifecycle-surface-parity | Existing-session Web environment control and unchanged fork | Bruno | Feature | Pending |

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

## Final Status

In progress. No PR readiness claim.
