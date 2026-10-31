# QA Run Report — 2026-10-31 — Cross-workspace session replies

- **Scope:** Issue #1102 / PR #1103; explicit replies, isolation, and transcript attribution.
- **Cadence tier:** targeted.
- **Build:** runtime `49a13e9c8`; Web uses the current PR working tree.
- **Environment:** isolated daemon home, socket, two workspaces, and headless browser. ACP mock provider is explicitly authorized for this run; no production sessions or desktop are used.
- **Status:** closed (targeted QA); PR verification continues.

## Personas and flows

Dora follows `J-session-collaboration` through native messaging and public CLI history. Bruno follows `J-13-follow-a-live-run` through the Web transcript. The runtime tour probes authorization boundaries; the Web tour checks truthful labels and failure text. This targeted walk does not repeat the historical reply-watch lifecycle scenarios.

## Session Matrix & Results

| Journey / scenario | Persona | Status | Evidence |
|---|---|---|---|
| RT-session-message-reply: project ping → explicit pong | Dora | Pass | Durable public history contains the ping, native reply result, and received pong. |
| RT-session-message-reply: isolation after reply | Dora | Pass | Five native probes return workspace_access_denied. |
| ET-web-session-message-card: live titles, workspaces, denied-send reason | Bruno | Pass | From, Sent and denied cards show the live title and workspace; denial survives reload. |

## Session Debriefs

The sender used its authorized workspace access to deliver ping. The receiver retained approve-reads permissions and answered using the persisted incoming message ID. The ordinary mutation approval was allowed once; no workspace access was granted. A separate history read confirmed pong at the sender.

The receiver then attempted an unsolicited send, forged reply ID, reply with attachments, reply with runtime override, and sender status read. Each requested ordinary workspace access; rejecting that request produced workspace_access_denied. The reply did not grant general access.

Artifacts remain under this task's isolated lab: `alpha-history-2.json`, `beta-probes-history.json`, and bootstrap manifest. Selected evidence: [pong receipt](../evidence/2026-10-31-issue-1102/alpha-history-2.json), [negative probes](../evidence/2026-10-31-issue-1102/beta-probes-history.json), [rendered transcript](../evidence/2026-10-31-issue-1102/reply-transcript.png), and [reloaded labels](../evidence/2026-10-31-issue-1102/rendered-labels.txt). Evidence follows the existing ignored sidecar convention.

## What Was Fixed

- Verified persisted reply provenance authorizes text delivery to the original sender in the same profile.
- Unknown target ownership is resolved before requesting label details.
- User destination API selectors and navigation owner IDs retain their distinct contracts.
- The thread rendering regression exposed structured error text being replaced by a generic adapter fallback; the adapter now preserves the nested message.

## Verification and limitations

Focused daemon reply authorization tests pass, including User destinations, target identity, attachments, and runtime refusal. The final scoped Web typecheck, 39 label tests, and both focused structured-error rendering cases pass. Current-head CI and both reviewer completion checks remain delivery gates. No full local gate, race suite, or local E2E suite was run.

## Final Status

PASS for the targeted project reply and label journey. User-scope navigation and reply variants are covered by owning unit/native suites, not a live User-scope lab. External providers and historical lifecycle cases were not rerun. Evidence audit passed. Manifest-driven teardown reported `TEARDOWN_ALL_CLEAN=true`; only the isolated lab and its owned processes were stopped. Final-head CI/review remain separate delivery gates.

## Runtime observations

Vite first-load preparation exceeded the initial browser timeout; retry after HTTP readiness succeeded. The preconfigured lab completed onboarding through its public endpoint before screenshot capture. No production runtime was used.
