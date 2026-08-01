# BUG-20260713-session-user-message-reorders-or-disappears: Authored session messages reorder or disappear

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-11/J-17, continue and reload a live agent session
- **Scenarios:** RT-session-message-reload; RT-session-prompt-idempotency
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** Live Cursor/Grok transcript reconciliation and reload replay

## Summary

All authored user messages could lose their original position while the live transcript reconciled. Ordinary messages rendered twice, with one optimistic copy moving below the assistant response. Reload repaired those duplicates, but a structured `/goal` command disappeared entirely because it had never been recorded as a durable user event.

## Regression — 2026-07-31

The opening prompt rendered twice when the first agent stream update arrived. The optimistic Assistant UI row and the durable transcript row still had different identities, so the thread treated one authored message as two entities. The remediation now carries the optimistic `message_id` through every ingress and stores it as the canonical transcript `UIMessage.id`; provider `user_message_chunk` echoes are rejected at ACP ingress, and retries are fenced by a separate durable `idempotency_key`. Isolated live settlement, exact cross-surface replay, conflict rejection, and cold reload passed on 2026-08-01.

## Reproduction

1. Open a live Cursor Agent session using `Grok 4.5 (High, Fast)`.
2. Send two ordinary prompts and wait for each response and live transcript reconciliation.
3. Observe the optimistic user rows during and after the assistant responses.
4. Send a valid `/goal` command and wait for Goal work to start.
5. Reload the exact session permalink.
6. Compare the authored inputs, response order, and durable transcript.

**Expected:** Every authored user message renders exactly once, remains immediately before the work it initiated, and survives reconciliation and reload with its exact text.
**Actual:** Ordinary user messages temporarily duplicated and moved after their responses. The `/goal` message also moved after its response, then disappeared on reload.

## Evidence

- Pre-fix session `sess-3fb644eedaea5ab9`; Goal run `looprun-afc234a9b50064b7`.
- Post-fix session `sess-59296138935045ea`; Goal run `looprun-3724bede0e0e62f5`.
- `/Users/pedronauck/dev/qa-labs/compozy-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/session-user-message-reload-fixed.json`
- `/Users/pedronauck/dev/qa-labs/compozy-automation-features-post-onboarding-fix-20260713-20260713-203513-816377-lab/qa-artifacts/qa/screenshots/session-user-message-reload-fixed.dom.txt`

## Fix

- **Root cause:** The Web reconciler had no identity shared between assistant-ui's optimistic user message and the daemon's canonical transcript event, so it appended unmatched runtime rows after the authoritative transcript. Structured Goal commands were dispatched before prompt-input recording, leaving no durable user event to replay. Hook-transformed input also conflated the provider-facing text with the exact text authored by the user.
- **Fix commit:** `a73b6587`
- **Regression test:** Existing canonical Web provider/read-model coverage now requires client-identity promotion without text matching and exact server chronology. Existing HTTP, UDS, core, Session Manager, and transcript suites require the AI SDK message ID to cross every boundary; recognized Goal commands persist once before dispatch; and transcript projection prefers `authored_text` while provider/audit input retains the hook-transformed technical text.

## Verification

- Fresh race runs passed for `internal/session`, `internal/transcript`, `internal/api/httpapi`, `internal/api/udsapi`, `internal/api/core`, and `internal/acp`.
- `make codegen-check`, `make lint`, Web typecheck, and the complete 3,410-test Web lane passed before live acceptance.
- A real post-fix Cursor/Grok session rendered two ordinary prompts and one `/goal` command exactly once before their corresponding assistant work throughout live reconciliation.
- Reloading the exact permalink preserved all three user messages exactly once. DOM offsets remained strictly chronological: `2744 < 2892 < 3005 < 3137 < 3230 < 3542`. The Goal was still approved and attached after reload.
- The user-requested second retest ran after the final daemon rebuild and global v2 migration. Browser reload completed in 874 ms; `QA-RELOAD-ONE-0714-0049`, `QA-RELOAD-TWO-0714-0050`, and the exact `/goal` command each remained present once, in that order (`orderPreserved=true`, `allExactlyOnce=true`).
- The 2026-08-01 isolated Codex retest admitted one Web prompt, returned the original stored turn for identical HTTP and CLI retries, and rejected divergent reuse with `prompt_idempotency_conflict` and `prompt_message_identity_conflict`. An independent history read retained one authored event with the original `message_id`; the canonical permalink rendered one user row and one `QA-OK` row after cold reload. Evidence: `/Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-20260801-040518-847041-lab/qa-artifacts/qa/`.
- The post-review production-parity rewalk used the current `web/dist`, session `sess-64ed9d39e1b551fb`, and a real Codex provider. Live settlement and cold reload each showed one Web-authored row and one response; an exact CLI replay retained `turn-ae0a591ae48f3fef` without another stream; independent history and DOM-count evidence stayed one-to-one. Evidence: `/Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-current-web-20260801-081126-391752-lab/qa-artifacts/qa/`.
