# Error registry

Generated from the shipped registry. A dash means the transport does not assign a code.

| Code                     | CLI exit | HTTP | Message                                                                                                     |
| ------------------------ | -------- | ---- | ----------------------------------------------------------------------------------------------------------- |
| daemon_unavailable       | 1        | —    | Proto did not start. See ~/.config/protohq/daemon/daemon.log.                                               |
| project_busy             | 1        | —    | <label> is busy with <command> (pid <n>). Try again when it finishes, or run proto dev to share one server. |
| project_offline          | 1        | 503  | <label> is not running. Start it with: cd <folder> && proto dev                                             |
| project_not_found        | —        | 404  | No project is registered as <label>.                                                                        |
| origin_not_allowed       | —        | 403  | Origin not allowed                                                                                          |
| state_unavailable        | 1        | —    | Proto cannot use ~/.config/protohq/daemon: <reason>.                                                        |
| daemon_version_mismatch  | 1        | —    | Proto is running a newer version than this project supports. Upgrade @protohq/cli in this project.          |
| billing_busy             | 4        | 409  | A seat change is in progress. Try again.                                                                    |
| billing_update_pending   | 4        | 409  | Stripe is confirming the seat change.                                                                       |
| billing_update_failed    | 4        | 503  | Billing update failed, try again.                                                                           |
| stripe_signature_invalid | —        | 400  | Invalid Stripe signature.                                                                                   |
| upload_incomplete        | 4        | 409  | Some files did not upload. Rerun the command.                                                               |
| publish_busy             | 4        | 503  | Proto publishing is busy. Try again in a minute.                                                            |
| cli_outdated             | 4        | 426  | Update @protohq/cli to use Proto Cloud.                                                                     |
| share_not_found          | 1        | 404  | No share with that name.                                                                                    |
| share_access_denied      | 1        | 403  | You do not have access to this share.                                                                       |
| share_paused             | 4        | 423  | Paused by the workspace owner.                                                                              |
| share_revoked            | 4        | 410  | This share is no longer available.                                                                          |
| share_disabled           | 4        | 410  | This share was disabled.                                                                                    |
| version_pruned           | 1        | 410  | This version is no longer retained.                                                                         |
| seat_required            | 4        | 403  | This action needs an Editor seat.                                                                           |
| plan_limit_reached       | 4        | 402  | The workspace plan limit was reached.                                                                       |
| snapshot_too_large       | 1        | 413  | Snapshots are limited to 50 MB, 5,000 files and 10 MB per file.                                             |
| capture_invalid          | 1        | 422  | A share thumbnail was refused; publish again with --no-captures.                                            |
| service_unavailable      | 4        | 503  | Proto Cloud is temporarily unavailable. Check https://status.protohq.design and try again.                  |
| login_interrupted        | 4        | —    | Login may have completed. Run proto login again.                                                            |
| device_code_expired      | 4        | —    | The login code expired. Run proto login again.                                                              |
| device_code_denied       | 4        | —    | The login was denied in the browser.                                                                        |
| workspace_slug_taken     | —        | 409  | That URL is taken.                                                                                          |
| workspace_slug_invalid   | —        | 422  | Use 3–40 lowercase letters, numbers and hyphens.                                                            |
| workspace_personal       | —        | 409  | Your personal workspace goes with your account.                                                             |
| subscription_active      | —        | 409  | Cancel the subscription in Billing first.                                                                   |
| confirm_mismatch         | —        | 422  | Type the workspace URL to confirm.                                                                          |
| member_not_found         | —        | 404  | Pick a member of this workspace.                                                                            |
| deletion_blocked         | —        | 409  | Resolve the workspace blockers before deleting your account.                                                |
| deletion_token_invalid   | —        | 400  | This link expired. Start again from Account settings.                                                       |
| legal_required           | —        | 403  | Accept the current legal documents to continue.                                                             |
| legal_version_mismatch   | —        | 409  | The legal documents have changed. Reload and try again.                                                     |
| legal_unpublished        | —        | 503  | Sign-up opens soon.                                                                                         |
| account_changed          | —        | 409  | This browser is signed in to another account now. Reload and try again.                                     |
| rate_limited             | —        | 429  | Too many requests. Try again later.                                                                         |
| request_resolved         | —        | 409  | This access request has already been resolved.                                                              |
| billing_unavailable      | —        | 503  | Billing is unavailable. Retry.                                                                              |
| photo_invalid            | —        | 422  | Use a square image under 512 KB.                                                                            |
| session_current          | —        | 409  | Use Sign out for this browser.                                                                              |
| workspace_not_found      | 1        | 404  | No workspace with that slug.                                                                                |
| forbidden                | 4        | 403  | You can't do this in this workspace.                                                                        |
| member_exists            | 1        | 409  | This person is already in the workspace.                                                                    |
| last_owner               | 1        | 409  | A workspace needs an owner. Transfer ownership first.                                                       |
| email_unverified         | —        | 403  | Verify your email before continuing.                                                                        |
| invitation_expired       | 1        | 410  | Invitation expired.                                                                                         |
| signup_closed            | —        | 403  | Sign-up on this staging instance is closed for this address.                                                |
| plan_required            | 4        | 402  | This needs Proto Pro.                                                                                       |
| not_found                | 1        | 404  | Not found.                                                                                                  |
| not_logged_in            | 4        | 401  | Run proto login to sign in.                                                                                 |
| network_unavailable      | 4        | —    | Can't reach Proto Cloud. Check your connection.                                                             |
| api_url_refused          | 1        | —    | PROTOHQ_API_URL must be a loopback URL or https://staging-api.protohq.design.                               |
| share_target_required    | 2        | —    | Choose one: a slug, --flow <id> or --all.                                                                   |
| share_no_invitees        | 2        | —    | Invite someone with --approver or --commenter.                                                              |
| share_dirty_tree         | 1        | —    | The snapshot contains uncommitted changes.                                                                  |
| share_target_not_found   | 1        | —    | No prototype or flow with that name.                                                                        |
| storybook_missing        | 3        | —    | storybook is not installed in <root>; install storybook and @storybook/react-vite.                          |
| storybook_unsupported    | 3        | —    | Storybook <version> is not supported; proto needs ~10.6.0.                                                  |
| story_not_found          | 1        | 404  | No story matches <id>.                                                                                      |
| invalid_input            | 2        | 400  | Invalid request input.                                                                                      |
| review_has_open_comments | 1        | 409  | Open comments remain; confirm before approving.                                                             |
| snapshot_stale           | —        | 409  | A new version is live. Reload to review it.                                                                 |
| internal_error           | 1        | 500  | The review operation failed.                                                                                |
| not_initialized          | 1        | —    | No proto.config.ts in <dir>. Run `proto init` (or pass --app <dir>).                                        |
| invalid_config           | 1        | —    | proto.config.ts is invalid.                                                                                 |
| not_a_vite_app           | 1        | —    | No Vite config found; pass --app <dir>.                                                                     |
| app_required             | 2        | —    | Several apps found; pass --app <dir>.                                                                       |
| port_in_use              | 1        | —    | Port <port> is used by another program; set PROTOHQ_PORT to move Proto.                                     |
| slug_taken               | 1        | —    | A prototype with this slug already exists.                                                                  |
| comment_not_found        | 1        | 404  | Comment not found.                                                                                          |
| message_not_owned        | 1        | 403  | Only the author can edit this message.                                                                      |
| attachment_too_large     | 1        | 413  | Images are limited to 5 MB.                                                                                 |
| attachment_type          | 1        | 415  | Only PNG, JPEG, WebP or GIF images.                                                                         |
| attachment_limit         | 1        | 422  | At most 6 images per message.                                                                               |
| attachment_expired       | 1        | 410  | The upload expired; attach it again.                                                                        |
| key_busy                 | 1        | 409  | The review key is busy; retry.                                                                              |
| cross_origin_write       | —        | 403  | Writes must come from the gallery's own origin.                                                             |
| ambiguous_credentials    | 4        | 400  | Send a session or a bearer credential, not both.                                                            |
| auth_required            | 4        | 401  | Sign in to review.                                                                                          |
| invalid_credential       | 4        | 401  | The credential is not valid.                                                                                |
| scope_denied             | 4        | 403  | Approval happens in the browser; credentials cannot approve.                                                |
| redirect_refused         | 4        | —    | The remote answered with a redirect; credentials are never re-sent.                                         |
| schema_mismatch          | —        | 503  | The review database schema does not match; apply the server database migrations.                            |
| playwright_missing       | 3        | —    | Install Playwright and its Chromium browser to capture prototypes or run stories.                           |
| migration_conflict       | 1        | —    | A target key exists with different content.                                                                 |
| migration_unverified     | 1        | —    | Migration verification failed.                                                                              |
| migration_source_changed | 1        | —    | The source changed during the run; put it in read-only mode, then re-run.                                   |
| source_not_read_only     | 1        | —    | The source accepted a write probe. Set PROTOTYPES_READ_ONLY=1 and drain requests first.                     |
| invalid_source           | 1        | 422  | Source paths must stay inside the app's source roots.                                                       |
| host_not_allowed         | —        | 403  | Open the gallery at 127.0.0.1 or add the host to dev.allowedHosts.                                          |

For local agent access, `plan_required` and `seat_required` include `allowance` and `next`. Follow the action and URL in `next`. `plan_required` can carry `allowance_exhausted` or `allowance_unavailable` in `reason`.

An agent check's `rate_limited` includes `retryAfter` in seconds. For `proto login --json`, `rate_limited` includes `retryAfterSeconds` in the error object.
