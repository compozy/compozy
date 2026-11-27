---
id: RT-computers-billing
area: RT
title: Buy Pro and recover a lapsed hosted route
persona: Iris
journey: J-buy-and-restore-remote-computers
expected: Stripe changes reach the correct account, computers, and retained hosted routes.
entry_points: Cloud purchase page; Settings Account; hosted hostname; local account stream
qa_status: pending
---

1. In a fresh browser without an enrolled installation, open the cloud upgrade page and continue with Google. Confirm annual billing is selected. Complete the fake Stripe Checkout, open the success page, and start CompozyOS with the same Google identity. Verify that Account shows Pro and the cloud purchase page now offers **Manage billing**.
2. Start checkout from a Free controller with remote access off. Connect to its account stream. Complete the fake Stripe Checkout and observe `account.changed` with `plan: "pro"` within ten seconds.
3. With three linked and enabled computers on Pro, make one the most recently used computer by opening a session on it. Simulate a failed renewal and confirm **Payment failed. Update your card.** appears while Pro is active. Recover the payment and confirm the message clears.
4. End the Stripe subscription. Check that the most recently used computer retains the Free slot and the other two show **Needs Pro**. Open a saved hosted hostname in a fresh phone browser and check that the SPA reports the plan requirement. Verify a hosted API request and a webhook delivery each return `402 plan_required`.
5. Restart the host the next day and revisit the saved hostname. From **Upgrade to Pro**, complete Checkout as the owner. Verify that the browser returns to the saved hostname. Run `compozy remote status --json` on the host. Verify that the same hostname returns to `ready`. Attempt the same upgrade as a different account and verify `computer_not_owned`.
6. Repeat the lapse and let seven days pass. Confirm the tunnel is deleted and a restart starts no hosted connector.

7. With an annual payment made five days ago, delete the account from a trusted admin device. Check the immediate sign-out, the full refund, and the cancellation before the computers are unlinked. Repeat with a payment more than fourteen days old and confirm cancellation with no refund.
8. Open monthly and annual Checkout from separate clients on the same Free account. Verify that Stripe expires the earlier link. Repeat with payment completing while Stripe is expiring the earlier link and verify the account becomes Pro without another payable link.
9. Repeat the in-window deletion with refunds temporarily unavailable. Verify Stripe cancels immediately. Restore refunds and confirm the deletion job completes the refund and continues account cleanup.
10. In staging, open **Manage billing** on the Pro account and confirm the portal allows switching between the staging monthly and annual prices, updating the payment method, and scheduling cancellation.
11. In the billing integration fixture, deliver an `invoice.paid` event for a subscription outside the configured Pro prices. Confirm the webhook acknowledges it and the account's billing records and plan stay unchanged. Move a known Pro subscription to a different Stripe price, deliver a subscription update, and confirm it removes the old entitlement. Repeat the price change with reconciliation as the trigger.
12. On staging, load the upgrade and Checkout success pages at phone and desktop widths. Confirm the respective artwork loads on both pages and the image requests return HTTP 200.

Use IT-111 for deletion and IT-060 through IT-066, IT-074, IT-082, IT-109, E2E-012, and E2E-013 as the integration evidence.
