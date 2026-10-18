---
title: Session messaging that holds up in long orchestrations
type: feature
---

Messages between agent sessions now reach long-running targets, stay behind the operator, and no longer exhaust a conversation after a few exchanges. (#760)

- Operator input always runs ahead of messages from other sessions, which have their own queue cap, so agents cannot fill the operator's slots. `queue_position` is the real dispatch rank.
- An agent's `mode: "interrupt"` is delivered as steer, and an agent steer never cancels another session's turn: when the target cannot take an injection, the message waits in the queue.
- `supersede: true` replaces your own older message still queued for the same target. Consecutive queued messages and reply wakes from other sessions run as one turn (up to 8), each kept as its own message; operator input is never merged.
- When a target has been in one turn longer than `session.messaging.queue_promote_after` (2 minutes by default), waiting agent messages are injected into the live turn. A failed injection falls back to after-turn delivery; nothing is dropped and nothing is interrupted.
- Hops count forwarding only: answering the session that started your turn, or replying with `in_reply_to`, keeps the hop, so a back-and-forth stays at hop 1. The flood guard is now a per sender→target rate budget; a send past it returns `message_rate_limited` (HTTP 429) with the limit and retry delay.
- `in_reply_to` links an answer to the original message. It never releases the sender's reply watch, so the completion wake still arrives.
- Reply watches now expire after 24 hours by default with a **No reply** outcome, so a forgotten watch no longer keeps its sender awake. Set `reply_watch_ttl = "0s"` to keep the previous never-expire behavior.
- `compozy session prompt` gains `--from-session auto|<id>`, `--in-reply-to`, and `--supersede`. Origin stamping is opt-in: without `--from-session`, a prompt stays operator input and a hint is printed on stderr. `--in-reply-to` and `--supersede` require a sending session.
- The operator can Steer (promote) an agent-authored queue row; editing it is still refused.
- The Web UI shows an **In reply** chip, groups consecutive messages from one sender, and says why a session needs attention.
- New `[session.messaging]` keys: `rate_limit_messages` (default 30, `-1` disables), `rate_limit_window` (default `10m`), `reply_watch_ttl` (default `24h`), and `queue_promote_after` (default `2m`, `0s` disables).

Migration notes: migration 00138 is applied automatically on daemon start and preserves all queued inputs and reply watches.
