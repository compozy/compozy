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

1. Use two signed-in installations and one linked computer. Run `compozy remote devices list --json` on the host before and after connecting from the second installation. Confirm that the device appears when its first remote session opens and that `last_seen_at` shows the host's connection time. Open a remote terminal from the second installation.
2. From a trusted admin device, remove the second device's terminal scope for the linked computer. On the host, use `compozy remote devices scopes <device> --set sessions:operate`. The terminal stream closes within two seconds with code `4503`. A fresh session can list sessions. Through the bridge, the terminal catalog returns `403 remote_scope_denied` with `details.scope` set to `terminal:operate`.
3. Disconnect the computer from Compozy Cloud and revoke the second device. Reconnect the computer through a different cloud replica. Confirm it reconciles the new snapshot before accepting a lease and the revoked device cannot obtain a new grant.
4. Repeat with the computer left disconnected. Existing remote streams end with close code `4402` when the last lease expires. On the controller, check that `compozy computer status <name> --json` reports `needs_attention` with diagnosis step `cloud`. Local work continues.
5. Turn off remote access with `compozy remote disable`. Check that the descriptor port refuses connections and the computer is still linked. Enable access, then run `compozy remote unlink` on the signed-in host. Check that its listeners close and its directory entry disappears.
6. Add the linked computer on two controllers with a working direct route and SSH forward. Keep a session stream open. Block the direct address and check that `compozy computer status <name> --json` reports the SSH route within ten seconds. Reconnect the stream through `/c/{id}` with its last event ID and check the transcript for gaps.

Walk this scenario after integrating the host and controller. Record timestamps and redacted event IDs in the QA report.
