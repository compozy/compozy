# BUG-20260929-rewind-offered-after-rewind: "Rewind to here" is offered on messages the daemon will refuse after an earlier rewind

- **Status:** open (pre-existing, outside session-continue-fork)
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Théo
- **Journey Step:** J-rewind-conversation: a second rewind after a rewind followed by new turns
- **Scenarios:** RT-conversation-rewind
- **Found:** 2026-09-29 · **Report:** docs/qa/reports/2026-09-29-session-continue-fork-r1-rewalk.md

## Summary

After one rewind and new turns, every user message still shows an enabled "Rewind to here". Confirming it answers `400 store: conversation rewind target is invalid`, and the Web shows the generic toast "Couldn't rewind this session. Refresh the conversation and try again." Refreshing does not help. The CLI (`compozy session rewind --message-id …`) gets the same refusal.

## Root cause

The refusal is intended: `conversationRewindTarget` (`internal/store/sessiondb/session_conversation_rewind.go`) refuses a target with archived events before it, and `sessions/lifecycle.mdx` states that "only rewind itself refuses those" (fork is allowed). The Web action gate does not know about the archived prefix, so it offers an action that always fails, and the 400 carries no machine code the toast could name. The code dates from #310, and this branch did not change it.

## Options

1. Expose rewind eligibility (for example a per-entry flag or the archived-prefix boundary in the transcript read) and hide or disable the action with a reason.
2. Give the refusal a code (`rewind_target_invalid`) and show "This message can't be rewound after an earlier rewind — fork from it instead."

Recommendation: 2 now (additive), then 1.

## Evidence

Lab session `sess-3e0e7af6f096ae49` (epoch 1). The CLI and HTTP both refused `msg-6bd22c10765c3ebb` and `msg-9e58379f79d9252a`.
