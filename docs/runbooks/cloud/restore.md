# Restore Compozy Cloud

Neon Launch retains seven days of point-in-time recovery. The hourly security log and nightly Postgres custom-format dump are stored separately in each environment's R2 bucket. The bucket lock `retain-30d` protects objects for 30 days. The lifecycle rule `expire-31d` expires them on day 31 and aborts incomplete multipart uploads after one day.

Set up the `cloud-backups-staging` and `cloud-backups-production` GitHub environments for unattended jobs on `main`. Bind the matching environment's `COMPOZY_CLOUD_NEON_DIRECT_URL`, `COMPOZY_CLOUD_R2_ENDPOINT`, `COMPOZY_CLOUD_R2_BACKUP_BUCKET`, `COMPOZY_CLOUD_R2_ACCESS_KEY_ID` and `COMPOZY_CLOUD_R2_SECRET_ACCESS_KEY`. Use the bucket-scoped credentials. Keep the production deploy reviewer on the separate `production` environment. Check the latest successful dump and hourly shipment during on-call handover.

## Cut over

1. Put the cloud custom domain in maintenance and drain both cloud replicas. Refuse writes and close existing control links before changing the Neon endpoints. Pause the scheduled backups so the archive writer stays on the original database until it has shipped the final batch.
2. Take a final security log shipment from the original branch. Run `compozy-cloud backup security-events` with the direct URL and R2 bindings. Save its RFC 3339 output as the archive cutoff. The command publishes coverage after every part is durable. Keep `security-events/` and `security-event-parts/` under the same retention and object-lock policy. If the original branch is unavailable and a verified archive cannot cover the last write window, keep ingress closed and account for the missing revocations before resuming service.
3. Restore the chosen point to an isolated Neon branch. For R2 recovery, download the protected dump over TLS and restore it to an empty isolated database with `pg_restore --exit-on-error --no-owner --no-acl`. Use the UTC time embedded in the dump object key as the conservative restore point. Apply current cloud migrations with `compozy-cloud migrate up`.
4. Set `COMPOZY_CLOUD_NEON_DIRECT_URL` to the restored direct endpoint. Keep the same environment's R2, Stripe and Cloudflare credentials. Run the finalizer with the original cutoff and chosen point.

   ```bash
   export COMPOZY_CLOUD_RESTORE_ARCHIVE_THROUGH="$archive_cutoff"
   compozy-cloud restore finalize --restore-point "$restore_point"
   ```

   The finalizer bumps every account's security epoch by 1,000, revokes device and web sessions and invalidates pending sign-in credentials. It removes account-token installations with post-point activity, reapplies archived revocations and deletion tombstones, replays Stripe and sweeps Cloudflare. Retry a failed attempt with the same point and cutoff. The epoch change is recorded transactionally and applies once.
5. Set the deploy environment's `COMPOZY_CLOUD_EXPECTED_RESTORE_POINT` to the chosen point. Point both pooled and direct Neon bindings at the restored branch and deploy the verified release while ingress is held. The service verifies the completion receipt before opening its listener. Remove the maintenance gate after readiness and a test computer reconnect confirm the new epoch. Resume backup jobs against the restored branch and record a fresh security log receipt.

Use an R2 dump within Stripe's 30-day event history window. Keep the failed database and receipts available for incident review under the normal retention policy.

Related evidence [Staging restore drill](../../qa/scenarios/RT-cloud-restore-drill.md).
