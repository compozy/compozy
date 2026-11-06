---
title: Provider usage works with several accounts and profiles
type: fix
---

If you run more than one Claude account through provider aliases (for example `claude-mm1` with its own `CLAUDE_CONFIG_DIR`), or use more than one profile, Provider usage now behaves correctly.

- **No more 429 storms.** A rate-limit reply with `Retry-After: 0` made CompozyOS retry right away and get rate-limited again. Every 429 now pauses that account for at least 5 minutes, longer if it repeats.
- **One request per account.** Each profile used to collect the same account separately. Collection is now shared per account, so two profiles no longer double the requests.
- **Aliases show their own account.** An alias whose launch command doesn't set its own account variable used to show the default account's quota. It now shows no usage instead of the wrong one.
- **Toggles and logos for aliases.** Settings → Provider usage lists every eligible alias with an on/off switch and its provider's logo, and you can edit the provider list while collection is off.
