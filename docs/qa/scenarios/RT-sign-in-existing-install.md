---
id: RT-sign-in-existing-install
area: RT
title: Preserve data and resume work after upgrading to mandatory sign-in
persona: Iris
journey: J-sign-in-and-use-compozy
expected: The upgrade holds new local work and preserves the existing home.
entry_points: CompozyOS 0.7.x home; scheduled jobs; local HTTP and Unix socket; CLI
qa_status: pending
---

1. Start with a 0.7.x home containing a workspace, session history, settings, a recurring job, and a one-shot job. Record their IDs and expected next occurrences.
2. Update to the official package. Confirm `pre_account_install: true` and check the CLI sign-in hint. While signed out, verify the recurring run history records `sign_in_required`, the one-shot is held, and webhook delivery returns 503 with `Retry-After`.
3. Sign in. Confirm the earlier workspace, history, settings and definitions are intact. Verify the one-shot runs once and the recurring job waits until its next occurrence.
4. With a live turn holding a lease and a terminal open, sign out. Verify the terminal WebSocket closes with `4401`, reconnect after sign-in to the same running terminal, and confirm the turn can complete its lease. Interrupt a turn by restarting while signed out and verify recovery waits for sign-in.

Use IT-094, IT-095, IT-097, IT-101, and IT-102 for the related automated evidence.
