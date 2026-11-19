---
title: Pair remote devices with a six-digit code and remove old devices
type: feature
---

Pairing a device for remote access now leads with a six-digit code. **Settings → Gateway → Pair a
device** shows the code grouped for reading, such as `123 456`, and copies it as six digits. The QR
code, the pairing link and the long pairing artifact are still available as secondary options. The
dialog now fits narrow phone screens without scrolling sideways.

At the pairing gate on the new device, type the six-digit code, with or without spaces or dashes, or
paste the pairing link or artifact. `compozy pair mint` prints the six-digit code next to the path
of the private artifact file, and `compozy pair redeem` accepts either one. The code is single-use,
shares the artifact's five-minute expiry, and redeeming either form uses up both. Repeated wrong
codes are rate limited per source address.

You can now remove a paired device instead of leaving it in the list forever. Use **Remove** on the
device's row in Settings → Gateway, or run `compozy device remove <device-id>`. Removing an active
device revokes it first and closes its live streams. Removing a revoked device deletes only its
record. Each removal appends a `gateway.device.removed` event before the record is deleted, so the
audit history remains. Revoke works as before. Agents can remove a device through the
`device_remove` action of `compozy__gateway`, and the HTTP and UDS route is
`DELETE /api/gateway/devices/{id}/record`.
