# Cloud admin and product news

## Cloudflare Access

Use separate Cloudflare Access application audiences for staging and production. Protect `/admin` and its descendants with the operator's Google identity. The cloud service repeats the issuer, audience, signature, expiry, and email allowlist checks.

Set these values on every cloud replica in each environment.

| Variable | Value |
| --- | --- |
| `COMPOZY_CLOUD_ACCESS_TEAM_DOMAIN` | The team's `<team>.cloudflareaccess.com` hostname |
| `COMPOZY_CLOUD_ACCESS_AUD` | The audience tag for the environment's admin application |
| `COMPOZY_CLOUD_ADMIN_ALLOWLIST` | Comma-separated operator email addresses |

Open the console at `/admin`. Action receipts are available in the security events table and through the receipt link returned with a download. Use the affected account's record for privacy requests. To resolve an identity conflict, paste the person's written request into **Detach identity** and keep credentials and IP addresses out of the request. Keep the downloaded account data in the support process for that request.

## Resend setup

Provision an isolated Resend team and a news segment for each environment. Contacts are team-wide in Resend, so use teams dedicated to these contacts. Keep the sending key scoped to the verified sender domain and use a separate Resend API key with Contacts API access.

| Variable | Purpose |
| --- | --- |
| `COMPOZY_CLOUD_RESEND_API_KEY` | Sends through Resend |
| `COMPOZY_CLOUD_RESEND_CONTACTS_API_KEY` | Creates, updates, and removes contacts in the environment's team |
| `COMPOZY_CLOUD_RESEND_NEWS_SEGMENT` | Receives opted-in contacts |
| `COMPOZY_CLOUD_MARKETING_POSTAL_ADDRESS` | The business postal address of Compozy, LLC, printed in every product news footer |
| `COMPOZY_CLOUD_UNSUBSCRIBE_KEY` | Stable 32-byte base64url key shared by the environment's replicas and broadcast command |

Create the text contact properties `compozy_unsubscribe_url` and `compozy_postal_address`. Route product news through the operator command below. Resend's Broadcast editor generates its own one-click destination. The Resend Send Email API accepts the signed Compozy Cloud URL and the two RFC 8058 headers.

## Send product news

Use the environment's cloud database and Resend settings. Verify the subject, plain-text message, and segment on staging before the production send.

```bash
compozy-cloud broadcast send --id release-0-8-0 --subject 'Remote Computers is available' --text-file ./news.txt
```

The command selects current opt-ins from Compozy Cloud and rechecks consent immediately before submitting each message to Resend. It adds the configured postal address and a signed unsubscribe link to the footer. Inspect the raw headers of a staging delivery and verify that the signed URL points to the same cloud environment used to send it.

If the command is interrupted, retry with the same ID, subject, and file within Resend's 24-hour idempotency window. Keep a new ID for a different broadcast. An interruption past the window requires checking which messages Resend already accepted before sending again.
