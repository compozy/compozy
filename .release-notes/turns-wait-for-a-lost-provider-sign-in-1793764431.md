---
title: Turns wait for a lost provider sign-in and then continue
type: feature
---

Switching accounts in a provider CLI, such as signing out of a Claude Code or Codex account near its limit and into another, used to fail every session that sent a prompt in that moment with `Authentication required`. You then had to resume each one by hand. Now a turn that loses the provider sign-in waits for it to come back and continues by itself, for every ACP provider.

- The session shows "Waiting for the provider sign-in…" with "Retrying automatically · Attempt N of M".
- Retries start after 15 seconds and double up to one a minute, for up to 10 minutes.
- A turn that hadn't started work is sent again as it was. A turn that had already started work gets a short note that the sign-in dropped and that it should check what is done before repeating anything. Your message is never recorded twice.
- If the sign-in doesn't come back in time, the turn fails as it did before. Later turns in that session then fail right away until one completes, instead of each waiting again. Cancelling the prompt ends the wait.
- Session reads show the cause in `runtime.recovery.reason` (`provider_auth` or `runtime_exit`). The existing `session.runtime_recovery.*` hooks also fire for these waits.

Configure it in the new global-only `[session.auth_recovery]` table (`enabled`, `window`, `initial_delay`, `max_delay`). It needs a restart. Set `session.auth_recovery.enabled = false` to fail right away, as before.
