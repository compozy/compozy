# BUG-20261002-native-approval-input-mismatch: A freshly approved native call rejects its own token

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-operate-workspace-context, approve a scoped operator tool invocation
- **Scenarios:** ET-native-workspace-scope-isolation
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

An operator cannot invoke a native tool with the one-shot approval just minted for that exact
request. Naming the target project by its registered name produces `approval_token_mismatch`, even
with identical caller project, profile, session, agent, tool, and input.

## Reproduction

- **Charter:** CH-native-workspace-handler-boundary · **Tour:** Feature Tour
- **Environment:** isolated macOS daemon, CLI over UDS, profile `studio`, en-US.

1. Create a Studio session for an agent whose permission mode requires explicit tool approval.
2. Run `tool approve compozy__workspace_info` with that session, agent, caller workspace ID,
   profile `studio`, and input `{"workspace":"Editorial"}`.
3. Immediately invoke the same tool with all of those fields unchanged and the returned token.

**Expected:** the token matches its approved request; ordinary remaining dispatch checks decide
whether the call can execute.

**Actual:** invocation fails with `tool_approval_required` / `approval_token_mismatch`.

## Evidence

- `docs/qa/evidence/2026-10-02-untested/scope-deny-after-tool-approval.json`
- `docs/qa/evidence/2026-10-02-untested/scope-bound-info-librarian.json`
- `docs/qa/evidence/2026-10-02-untested/scope-identities.json`
- `docs/qa/evidence/2026-10-02-untested/scope-walk-ended.json`

The approval receipt and invoke result are captured together with raw tokens redacted. The first
driver attempt read the wrong nested token field and made no invocation; the evidence above is the
subsequent actual approve/invoke pair, not that driver error.

## Fix

- **Root cause:** approval issuance hashes the submitted input, but dispatch binds native workspace
  references and re-encodes the object before consuming the approval. The store therefore compares
  the raw alias digest with the canonical workspace-ID digest for the same request. An inherited
  operator workspace also previously needed a second binding pass to reach its durable identity.
- **Repair:** the booted approval issuer uses the same input binder as dispatch. Inherited operator
  workspace input becomes canonical on the first pass. Supplied digests are validated before
  binding; digest-only requests retain their bound-input meaning. Approval consumption remains
  after hooks and checks the final input and all existing scope, expiry and single-use fields.
- **Fix commit:** `bef9a13b8`
- **Regression test:** daemon composition owns the invariant in `TestDaemonBootToolRegistry` in
  `internal/daemon/native_tools_test.go`. Four cases reproduced the approval mismatch before repair:
  name, path, omitted workspace and absent input. The suite also covers submitted digests, digest-only
  approval and rejection of an inconsistent supplied digest. Existing store/dispatch suites retain
  ownership of token scope, replay and hook sequencing. No new test file was added.

## Verification

Fresh session `sess-0fcc42a0ca0704c4` verifies the repaired operator flow on the rebuilt daemon:

- Name, path, omitted workspace and absent input each approve and complete for the intended project.
- A valid submitted digest and a digest-only approval both complete. An incorrect digest is rejected.
- Changing the destination rejects the token without consuming it; the original input then completes,
  and a subsequent reuse returns `approval_token_replayed`.
- An HTTP-issued approval is consumed over UDS, and a UDS-issued approval is consumed over HTTP.
- The session is stopped and independently confirmed stopped through its scoped HTTP detail.

Evidence: `approval-input-red-owning.log`, `approval-input-green.log` (26.326s, race-enabled),
`approval-input-gate.log` (all affected local lanes passed), `approval-fixed-*.json`, and
`approval-test-conventions.txt` (zero findings) in this cycle's evidence directory. Raw tokens are
redacted before evidence is written. Initial fixture wiring failures are retained as diagnostics;
the owning red log isolates the four actual behavior failures.

Two attempts to create the additional live hook fixture returned `hook_validation_failed`; no hook
was installed. That interaction is still under investigation and is not claimed as live evidence.
The CLI's session selector remains an operator invocation, so these results do not establish the
separate agent-only cross-workspace boundary. Row 6 remains Pending for that hosted-native walk.
