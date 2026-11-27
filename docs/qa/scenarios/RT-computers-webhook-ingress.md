---
id: RT-computers-webhook-ingress
area: RT
title: Confirm and deliver a webhook through a computer
persona: Bruno
journey: J-operate-remote-computers
expected: Each signed delivery reaches its bound trigger on the owning computer and workspace, and a base URL change requires confirmation before delivery resumes.
entry_points: Automations → Triggers; POST /api/ingress/bindings; Webhook HTTPS endpoint
qa_status: pending
---

1. Link a computer, turn on remote access, and enable a hosted route. Create two workspace webhook
   triggers with the same slug. Confirm each advertised URL and verify the workspace ID and stable
   webhook ID distinguish the two paths.
2. Send a signed delivery to one URL. Verify exactly one attributed run on the intended computer
   and workspace. Repeat the same delivery ID and verify it does not start a duplicate run.
3. Change the advertised base. Check both triggers show **Needs confirmation** and an unconfirmed
   delivery is refused. Confirm the new address through Automations or the ingress bindings API.
   Deliver again using an unused delivery ID and observe the run.
4. Repeat with an operator-owned HTTPS tunnel forwarding to the loopback listener and
   `remote.ingress.public_url` set. Check that the trigger path stays stable across the route change.
   Declare an `X-Compozy-Claim-Token` trailer on a test delivery and expect `400 claim_token_not_remote`.
   Resend that delivery ID without the declaration and confirm the run starts.
5. Disable remote access while a delivery is in flight, inspect the resulting state on the owner,
   then retry according to the sender's redelivery rules after the route returns.

IT-075 and IT-078 cover routing and address confirmation in the integration suite. Walk on the
integrated release build.
