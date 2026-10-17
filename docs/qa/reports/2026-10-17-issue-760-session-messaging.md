# Issue #760: session messaging delivery — scoped real-lab walk

## Setup

- Binary built from branch `fix/issue-760-session-messaging-delivery` (including review remediation).
- Isolated QA lab (own `COMPOZY_HOME`, HTTP port, UDS socket) bootstrapped with `eng-qa-bootstrap`;
  torn down clean (`teardown.json` `clean: true`).
- Live provider: Claude `claude-haiku-5-5`, low effort, native login. Sessions created through the CLI;
  agent-origin sends through `compozy session prompt --from-session <id>`, agent-identity HTTP over UDS
  (`steer`/`interrupt`), and the native `compozy__session_prompt` tool called by a live agent
  (`notify_on_complete`).
- Lab overrides: `queue_promote_after = "15s"`, `reply_watch_ttl = "1m"`, `rate_limit_messages` 6 then 40
  per `2m`. The daemon was restarted once mid-walk (migration 00138 applied, new sessions worked).

## Results

| Scenario step | Result |
|---|---|
| RT-session-message-origin 9: human-first order and rank | PASS. Three agent messages then one human prompt queued behind a busy target: the human reports `queue_position` 1, runs first in its own turn. |
| RT-session-message-origin 9: separate caps | PASS. Ten agent entries fill the agent pool, the eleventh returns `session_queue_full`, an operator prompt is still admitted at #1. |
| RT-session-message-origin 10: agent steer/interrupt | PASS. An agent steer is injected into the live turn without interrupting it; an agent interrupt reports mode `steer`. |
| RT-session-message-origin 11: supersede | PASS. `canceled_queued_entries: 2`; another sender's entry stays; dropped markers emitted. |
| RT-session-message-origin 12: batching | PASS. Queued agent messages dispatch as one turn with one attributed input event each; ten entries dispatch as 8 + 2. |
| RT-session-message-origin 13: promotion | PASS. Queued agent messages are injected into a live tool-running turn after 15s with their original message ids; with an operator prompt waiting nothing is promoted. Operator Steer of an unchanged agent row is injected and keeps the message id and sender; edit returns `input_agent_authored`. |
| RT-session-message-origin 14: hop and rate | PASS. `in_reply_to` answer keeps hop 1, a conversational answer keeps hop 1, forwarding B→C is hop 2. The seventh send in the window returns `message_rate_limited` with `limit`, `window` and `retry_after_seconds`. |
| RT-session-message-origin 15: CLI origin | PASS. `--from-session <id>` stamps the sender; `--supersede` and `--in-reply-to` without it are refused; HTTP operator `supersede` returns 400. |
| RT-session-message-reply 13: TTL expiry | PASS. A watch armed by a live agent behind a busy target expires after ~72s (`expired`, delivered once); the sender wakes with the expiry text; the later consumption of the message produces no second wake. |
| ET-web-session-message-card / origin 16: Web | PASS. Consecutive inbound cards from one sender collapse ("qa-C · 2 messages"), the In reply chip and No reply outcome render, the queue strip lists the human at #1 and agent rows as "From {title}" with Steer and without Edit. |

Evidence (local sidecar `docs/qa/evidence/issue-760/`, published copies linked from the PR):
`j1-b-grouped.png`, `a-inreply.png`, `b3-queue-strip.png`.

## Not walked here

- The Needs-attention reason rendering is owned by the web suite (`session-badge.test.ts`).
- Origin steps 3, 5 and 8 (restart before dispatch, rename retry, v4 upgrade) are unchanged by #760 and
  remain owned by their automated suites.
- The strict real-scenario audit reports FAIL because its feature-profile behavioral criteria (task roots,
  four-actor charter, disruption probe) belong to the real-scenario playbooks, not this scoped walk.
- Heavy local gates were not run per operator directive; the PR CI is the gate.
