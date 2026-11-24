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
5. Restart the host the next day and revisit the saved hostname. From **Upgrade to Pro**, complete Checkout as the owner and confirm the browser returns to that hostname. Attempt the same upgrade as a different account and verify `computer_not_owned`.
6. Repeat the lapse and let seven days pass. Confirm the tunnel is deleted and a restart starts no hosted connector.

7. With an annual payment made five days ago, delete the account from a trusted admin device. Check the immediate sign-out, the full refund, and the cancellation before the computers are unlinked. Repeat with a payment more than fourteen days old and confirm cancellation with no refund.

Use IT-111 for deletion and IT-060 through IT-066, IT-074, IT-082, IT-109, E2E-012, and E2E-013 as the integration evidence.
