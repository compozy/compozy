---
id: RT-cloud-restore-drill
area: RT
title: Restore cloud state without reviving revoked access
persona: Cloud operator
journey: J-cloud-restore
expected: The staged cloud stays in maintenance until the finalizer reapplies security changes and reconciles Stripe and Cloudflare.
entry_points: Neon staging; Compozy Cloud operations CLI; staged computer control link; Stripe test mode; staging R2
qa_status: pending
---

1. In staging, seed a synthetic account, two devices, a web session, a linked computer and a test Pro subscription. Prepare a remote grant and record the starting account epochs and Neon restore point.
2. Revoke one device, cancel the test Stripe subscription, and provision a new hosted tunnel after the restore point. Verify the revocation in the security log. Put staging into maintenance and ship the final batch to R2.
3. Restore the isolated point, migrate, and try to start the service with the expected restore point configured. Confirm startup refuses before a successful finalization.
4. Interrupt the finalizer after the database changes by making the Stripe or Cloudflare dependency temporarily unavailable. Check that ingress remains held. Restore the dependency and retry with the same arguments. Every account should have advanced its epoch by exactly 1,000.
5. Verify a formerly valid device and web session are revoked, the revoked device is still refused, and the retained device can sign in with its existing key. Confirm the Stripe subscription is `canceled` and the post-point tunnel is gone.
6. Reopen staging. Connect the linked computer and read its signed snapshot at the new epoch. Confirm the revoked device is absent. Verify a mint with the pre-restore grant is refused and a freshly minted grant works.
7. Record the restore point, archive cutoff, revision and provider results. Repeat before launch and quarterly using the restore runbook and IT-107.

[Cloud restore runbook](../../runbooks/cloud/restore.md)
