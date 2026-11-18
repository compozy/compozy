---
title: Sessions recover after you sign in to the provider again
type: fix
---

A session no longer stays stuck after its provider account signs out. Before, signing out of a provider CLI such as Claude Code while a session was open made the next turn count through the provider's retry attempts and then hang, even after you signed back in and **Usage** showed the new account; only restarting the daemon brought the session back.

Now a sign-in failure leaves the session waiting for you to sign in. Once the credentials are valid again, the next prompt rebuilds that session's provider process and resumes the conversation, and a provider sign-in check or **Usage** refresh that sees valid credentials recovers the matching sessions too. Other sessions and providers are not touched, and the daemon keeps running.

Migration notes: no action needed.
