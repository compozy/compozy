# BUG-20260928-native-fork-clone-load-refused: A Codex native fork child cannot run while its source is alive

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Rafa
- **Journey Step:** J-15-operate-session-via-cli-api, derive branch (fork an idle Codex session, prompt the child)
- **Scenarios:** RT-session-derive-native-fork
- **Found:** 2026-09-28 · **Report:** docs/qa/reports/2026-09-28-session-continue-fork-exec-b1.md

## Summary

Rafa forks an idle, bound Codex session (`compozy session fork <src>`). The fork returns `seed native_fork · pending`.
Every prompt to the child then fails with `acp: load session failed … {"code":-32603,"message":"Internal error","data":{"details":"thread <clone> already has an active writer"}}`
and the child stays `native_fork · pending` for as long as the source session's Codex process is alive. The child is unusable
until the operator happens to stop the source.

## Reproduction

1. `compozy session new --agent cx-agent` (provider `codex`, codex-acp 2.0.0), one prompt, wait idle.
2. `compozy session fork <src> -o json` → `seed: native_fork`, `native_state: pending`.
3. `compozy session prompt <child> "Which codeword was mentioned earlier?"` → error above; repeat → same error.

Evidence: `docs/qa/evidence/2026-09-28-session-continue-fork-b1/cx-fork.json`, `docs/qa/evidence/2026-09-28-session-continue-fork-b1/cx-fork-p1.txt`, `docs/qa/evidence/2026-09-28-session-continue-fork-b1/codex-acp-fork-close-probe.txt` (a direct stdio probe: the clone stays locked
in the forking process even after `session/close`; it loads once that process exits).

## Root cause

External half: codex-acp keeps the forked thread's writer in the process that ran `session/fork` until that process exits.
Product half: `launchPromptRuntimeCandidate` (`internal/session/derive_native_bind.go`) only settled the native bootstrap
`failed` (and replayed the carried context) for `resource not found` / `load unsupported`; any other `session/load` refusal
surfaced as a prompt error and left the child pending forever. ADR-003 (Risks) requires "an id its fresh process cannot load →
`native_state = failed`, imported-context replay".

## Fix

`nativeCloneLoadFailed` in `internal/session/derive_native_bind.go`: a native bootstrap whose `session/load` fails for any reason
(or whose agent lacks load) settles `failed` with the redacted `session/load: …` text and restarts on the carried context; a
canceled caller and failures before `session/load` still surface. Regression: `TestForkNativeSeed/Should_settle_failed_and_replay_the_carried_context_when_the_clone_cannot_load`
(`internal/session/manager_derive_test.go`, new "clone held by the forking process" case — fails before, passes after).
A `session/close` of the clone in the source process was tried first and reverted: the probe proved it does not release the writer.

## Verification

Re-walk on the rebuilt lab daemon: fork of the live Codex source → child prompt answers `PELICAN-42` from the carried context;
`session status` prints `Derivation  seed native_fork · failed: session/load: internal error (replayed the carried context)`
(`docs/qa/evidence/2026-09-28-session-continue-fork-b1/cx-fork3.json`, `docs/qa/evidence/2026-09-28-session-continue-fork-b1/cx-fork3-p1.txt`). OpenCode and Claude native forks still load (`docs/qa/evidence/2026-09-28-session-continue-fork-b1/oc-fork*.{json,txt}`, `docs/qa/evidence/2026-09-28-session-continue-fork-b1/cl-fork*`).
