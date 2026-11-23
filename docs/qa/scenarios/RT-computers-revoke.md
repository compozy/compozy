---
id: RT-computers-revoke
area: RT
title: Revoke or narrow a Remote Computers device
persona: Iris
journey: J-operate-remote-computers
expected: A device loses remote authorization after revocation or scope narrowing, including when its computer reconnects to a different Compozy Cloud replica.
entry_points: Settings → Account → Devices; remote session and terminal streams; Compozy Cloud control link
qa_status: pending
---

1. Use two signed-in installations and one linked computer. Open a remote terminal from the second installation.
2. From a trusted admin device, remove the second device's terminal scope for the linked computer. The terminal stream closes within two seconds. A fresh session can use its remaining scopes.
3. Disconnect the computer from Compozy Cloud and revoke the second device. Reconnect the computer through a different cloud replica. Confirm it reconciles the new snapshot before accepting a lease and the revoked device cannot obtain a new grant.
4. Repeat with the computer left disconnected. Existing remote streams end when the last lease expires. Local work continues.

Walk this scenario after integrating the host and controller. Record timestamps and redacted event IDs in the QA report.
