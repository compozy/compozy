# Handle abuse, suspension and privacy requests

Use `support@compozy.com` and GitHub Discussions for support. Receive hosted-route abuse reports at `abuse@compozy.com`. Handle access, export and deletion requests from `privacy@compozy.com` within 15 days.

For an abuse report, verify the hostname and account from the Access-protected admin console. Delete a hosted tunnel through the console's generation-fenced action. Repeated abuse can justify suspension. Confirm device and web sessions and tokens are revoked, connected computers receive the empty snapshot, and the tunnel enters deletion. Unsuspend after review if appropriate. The account has to sign in again. Verify the action's security event and preserve the case reference in the operator's record.

For a privacy request, verify the requester's identity in the privacy mailbox. Use the admin export or account deletion action as appropriate. Send plain-text status replies from the cloud's Resend sender. Use a fresh reply ID for each separate message and reuse the same ID when retrying a delivery.

```bash
compozy-cloud email privacy-reply --request-id "$reply_id" "$private_json_file"
```

Keep the file private. It contains `to` and `text`. Supply export material by the approved privacy channel and keep session credentials and provider secrets out of the reply.
