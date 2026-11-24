---
id: RT-computers-upgrade
area: RT
title: Upgrade a home that used the Remote Gateway
persona: Iris
journey: J-operate-remote-computers
expected: The upgrade preserves local work, app tokens, and webhook bindings and explains the remote setup changes once.
entry_points: compozy daemon start; compozy account login; Settings → Computers; local HTTP and UDS
qa_status: pending
---

1. Stop a v0.7.8 daemon and use a disposable copy of its home. Include a project, sessions, a
   disabled automation job, and app tokens with browser origins. Prepare a second copy with two old
   connection profiles, credentials, paired devices, and bound webhook triggers.
2. Start the v0.8.0 release runtime. Confirm the account reports signed out and identifies a
   pre-account installation. Check that the profiles and credentials were removed from the second
   home and that the log records one `config_keys_removed` warning.
3. While signed out, invoke `compozy gateway status` and `compozy connect list`. Both exit 2 and
   point to `compozy computer`. Sign in and verify the same project, session titles and IDs, job
   definitions, and settings.
4. Confirm the first home has no upgrade notice. On the second, open Settings → Computers from the
   old `/settings/gateway` URL. Verify **Learn more** and, when bindings need confirmation,
   **Review** and the correct count. Use **Dismiss**, reload, and restart. Check that HTTP and UDS
   return the same dismissed state.
5. Authenticate with an existing app token from its allowed browser origin, then verify refusal
   from a different origin. Add a remote computer again and follow `RT-computers-webhook-ingress`
   for webhook delivery at the new address.

IT-001 through IT-003, IT-097, and E2E-009 own the automated upgrade coverage. Walk the
scenario on the integrated release build.
