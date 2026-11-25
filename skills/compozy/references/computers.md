# Remote Computers

## Compozy Cloud

`https://cloud.compozy.com` serves the Remote Computers control plane. Agent sessions and their data stay on their computers.

## Use a linked computer from the controller

- Discover linked computers with `compozy computer available --json`, add one with `compozy computer add account <id>`, and inspect routes with `compozy computer status <name|id> --json`.
- Target a computer through the local Unix socket with `compozy session list --computer <name|id> --json`. Select the local daemon with `--computer local`. Machine-local verbs return `flag_not_applicable` if you pass the flag.
- Send a prompt with `compozy session prompt <session_id> --computer <name|id> "<message>"`. The host acknowledges acceptance and continues the turn after the controller disconnects. Read the reply with `compozy session history <session_id> --computer <name|id> --json`. Use `-o jsonl` on the prompt command to follow the live stream.
- Use `GET /api/computers` for the registry and `GET /api/computers/stream` for `computer.changed`. Reconnect with `Last-Event-ID` or `after_sequence`; refetch after `stream_reset`.
- Forward an operator API call through `/c/{computer_id}/api/…` on local HTTP or UDS. The registry and event cursors belong to the active account. The controller injects the remote proof, rejects requests carrying claim tokens, and refuses to chain through another controller.
- Use the target's operator routes for files, Changes, worktrees, Tasks, the scheduler, settings, extensions, provider status, and Vault metadata. The target returns `remote_scope_denied` with the required scope in `details.scope`. Open and attach to a terminal with the target workspace and `terminal:operate`. Reconnect a live terminal with a fresh attach ticket and its last output cursor.
- To execute a task there, enqueue it with `POST /api/tasks/{id}/runs` through the bridge. Prompt an agent session on the target to call `compozy__task_run_claim_next`. The lease owner calls the heartbeat and completion tools locally on that computer. Read the run through `GET /api/task-runs/{run_id}`.
- A tool descriptor's `remote_scope` is the scope required for a remote operator invocation. Send `workspace_id` to select a target workspace. The destination applies its operator policy. Supplying `session_id`, `agent_name`, `turn_id`, or `approval_token` returns `remote_caller_override_forbidden`.
- The controller bridge accepts local operators. It returns `app_token_route_forbidden` for app tokens and `agent_scope_denied` for identified agent callers. The controller renews the device credential five minutes before expiry on the current tunnel.
- Account, controller registry, agent-kernel, and computer link and unlink routes return `route_not_remote` through a tunnel. A second bridge hop returns `remote_bridge_not_transitive`.
- `compozy computer remove <name|id>` removes the local entry. `--unlink` also frees the account slot. Use `compozy computer routes <name|id> remove <kind:address>` to remove a learned address.

Use `compozy__computer_list` for the registry and local execution settings. Pass `id` to `compozy__computer_status` and `compozy__computer_remove`. Add a linked computer with `compozy__computer_add` using `{"kind":"account","id":"<id>"}`.

Call native tools with a declared `remote_scope` using an optional `computer` name or ID. `compozy__session_list` with `{"computer":"devbox"}` lists sessions in User on the destination. Supply the destination's workspace name or ID in `workspace` to list a project. The source session policy, pre-call hooks and approvals apply before the destination checks the device's scopes and its own operator policy. The result is the destination's result and `_meta.computer_id` identifies where it ran. A session confined to workspace paths receives `tool_forward_denied`. Task leases and controller-local tools return `tool_not_forwardable` when given `computer`.

## Automatic session placement

- Read `GET /api/host/resources` locally or through a computer's bridge. It reports logical CPUs, CPU utilization, physical memory in bytes, and a sample timestamp. Auto uses samples up to 15 seconds old and falls back to weight when all eligible computers lack usable samples.
- Update Auto and local execution with `PATCH /api/computers/local`. The local settings also appear in `GET /api/computers`. Add a remote computer before turning off local execution. `local_execution_disabled` blocks new local sessions, Loop runs, and automation runs. Existing records remain readable.
- Set weights on `/api/computers/local` or `/api/computers/{id}` with `load_balancing_weight`. Prefer is `100`, Normal is `50`, Less often is `25`, and Manual only is `0`.
- Preview Auto with `POST /api/computers/placement`, using the draft workspace in `workspace` and its computer in `project_computer_id` if it belongs to a linked computer. With `computer` set to `auto`, `POST /api/sessions` selects the matching repository workspace again and returns the chosen `computer_id`. `no_eligible_computer` means the draft cannot be placed.
- A computer with a live route remains eligible for Auto when an SSH service warning sets its display status to `needs_attention`.

| Endpoint                                       | Purpose                                               |
| ---------------------------------------------- | ----------------------------------------------------- |
| `GET /healthz`                                 | Check replica readiness                               |
| `GET /v1/jwks` or `GET /.well-known/jwks.json` | Read the cloud's public Ed25519 verification keys     |
| `GET /v1/gate-policy`                          | Fetch the signed sign-in policy in compact JWS format |

Treat key IDs in the cloud JWKS `retired` list as revoked. A computer closes remote sessions minted with a retired key. Verify the gate policy signature against the cloud JWKS. A grace policy applies only before its `until` deadline. For `429 rate_limited` and `503 cloud_unavailable`, honor the `Retry-After` header.

## Set up an SSH computer

Run `compozy computer add ssh <host> --json` to start a setup and get its ID. An operator can use the interactive CLI to confirm a host key, or pass a separately verified fingerprint with `--trust-host-fingerprint`. Read host suggestions from `GET /api/computers/ssh-hosts`.

Inspect `compozy computer setup status <setup_id> --json` or long-poll `GET /api/computers/setups/{setup_id}?wait_ms=30000`. A `trust_host` prompt contains only the host and its fingerprint. For `needs_operator_secret`, direct the operator to the app or `compozy computer setup answer <setup_id>` from a TTY. The operator's password or code goes directly to SSH.

Start SSH setup with `compozy__computer_add` using `kind` set to `ssh` and `host` set to the host. Follow progress with `compozy__computer_setup_status` and `wait_ms` up to `30000`. Use `compozy__computer_setup_answer` with `setup_id` and `accept` to relay the operator's host-key decision. Enter SSH secrets through the app or the CLI.

Use `compozy__computer_setup_cancel` with `setup_id` on the controller where setup started. The CLI `compozy computer setup cancel <setup_id>` and the local DELETE route also release the slot. The tool requires approval in `approve-edits` and `auto` because cancellation ends the setup permanently. `computer_setup_linked` means the host already has a link. Manage it through the computer registry. Repeat the add command to resume an interrupted setup. Use `compozy computer update <name|id>` for a managed SSH upgrade.

## Local account

Use `compozy account login` to sign in through the browser. Show the activation URL and code to the operator and let them approve the grant. Use `compozy account status --json` or `compozy__account_status` to read the account, plan usage, and current device. Use `compozy account logout` to end the session on this computer. `compozy account upgrade --annual` opens annual checkout when billing is available.

Release builds report a local gate of `enforced`, `grace`, or `off` in `GET /api/account`. If an operator action returns `sign_in_required`, direct the operator to `compozy account login`. During grace, local work can start until the reported deadline. Computers, devices and billing still require an account. For a linked host, sign out with `compozy account logout --unlink`.

Use `compozy status --json` to check the local daemon while signed out.

On CI, provide a named account token through the launcher's `COMPOZY_ACCOUNT_TOKEN` secret every time the daemon starts. The daemon keeps the token in memory for its own lifetime. Operators create one with `compozy account token create --name ci`. Keep its one-time value out of logs and tool results. The only cloud endpoint that accepts it is `POST /v1/account-tokens/exchange`. Remote and billing operations return `account_token_local_only`.

Local HTTP and UDS expose the same `/api/account` routes for sign-in, status, devices, approvals, and billing. Subscribe to `GET /api/account/sign-in/events` before starting a login. It carries attempt progress and state without account details. Follow `GET /api/account/stream` after sign-in. On `stream_reset`, read current status and approvals before following changes again.

Use `POST /api/account/checkout` with `{"interval":"year"}` or `{"interval":"month"}` and give the operator its Stripe URL. Follow `account.changed` for `plan` and `payment_issue`, including when remote access is off. `POST /api/account/billing-portal` returns the URL for payment methods, interval changes, and renewal cancellation. Use `GET /api/account` for the current subscription and `refund_eligible_until`.

A cloud outage leaves an existing sign-in intact. Retry account actions after the response's `Retry-After` when present. The account tool and routes are local to the installation.

Pass `marketing_opt_in: true` in `POST /api/account/login` only when the operator explicitly opted in. Use `PATCH /api/account` with `marketing_opt_in` for settings changes. `GET /api/account` lists seven days of `recent_sign_ins`. `POST /api/account/sign-in-notices/dismiss` clears them for the current installation.

On an admin device, `DELETE /api/account` takes `confirm_email` and returns `202` with `state: "deleting"`, `subscription_cancelled`, and `refund`. It signs out the installation. The cloud cancels Pro and applies the refund rule before removing linked computers and account data. During suspension, account operations return `account_suspended` and cloud credential checks return `reauth_required` with that reason.

## Cloud sign-in

Start a device grant with `POST /v1/device-flow`. Show the operator the returned user code and `verification_uri_complete`. The operator compares the code with `/activate` and completes Google or GitHub sign-in in the browser. Poll `POST /v1/device-flow/token` at the returned interval. Add five seconds on `slow_down`, and stop on `access_denied` or `expired_token`. Cancel a superseded attempt with `DELETE /v1/device-flow/{device_code}`.

Enroll with `POST /v1/devices/enroll` using the single-use enrollment token and a DPoP proof from the per-account device key. `POST /v1/session/refresh` rotates the refresh credential. `POST /v1/session/logout` revokes that session and keeps its enrolled device. Keep device codes, enrollment and refresh credentials, and OAuth callback URLs out of tool results and logs.

Use the Google activation page's **Recover admin access** action after losing the last active admin device. The provider must verify a fresh sign-in.

## Cloud account and computer access

- `GET /v1/devices` lists the account's devices. An admin can promote or demote another device, edit its scopes, or revoke it with the device routes. Each trust change increments the account's security epoch.
- `GET /v1/computers` lists the account's linked computers. Use `POST /v1/computers/reservations` to reserve a slot before SSH setup. On resume, send `previous_reservation_id` again. Release the reservation when setup is cancelled. A link token can be redeemed again by the same computer key to recover the original link result.
- Request a fresh grant for each connection attempt at `POST /v1/computers/{id}/grants` with the route kind. Use the bound device key. Free grants cover `direct` and `ssh`. Compozy Pro also covers `cloud`.
- Follow `GET /v1/account/events` for device, computer, plan, and approval changes. After a reconnect, read the current state. A decision accepted for delivery can be checked at `GET /v1/approvals/{id}` until the computer provides its `applied_result`.
- When Pro ends, the account keeps its most recently used enabled computer on direct or SSH. Hosted access is gated at once and the hostname is retained for seven days. The owning account can buy Pro at `https://cloud.compozy.com/billing/upgrade?hostname=<host>` and return to the hosted URL.

Keep link tokens, grants, computer key proofs and private keys out of logs and tool results.

## This computer

Use `compozy remote link` to link this computer and turn on its remote access. Use the printed activation URL and user code if sign-in is needed. SSH setup pipes the single-use link token to `compozy remote link --token-stdin`.

Read status with `compozy remote status --json` or `compozy__remote_status`. The tool is local to this computer and has no remote scope. Use `compozy remote disable` to close the listeners and keep the account link, `compozy remote enable` to reopen them, and `compozy remote unlink` to remove the account link and directory entry. A linked host revalidates its account link daily, including when its remote listeners are off.

The same operations are available over HTTP and UDS under `/api/remote`. Use `compozy remote devices list --json` to read effective scopes for devices that opened a session or requested approval on this computer. Narrow one device with `compozy remote devices scopes <device> --set <scopes>`. Approve or deny a pending key with `compozy remote devices approve|deny <device>`, using `--approval-id` to choose among multiple pending keys. `compozy remote devices revoke <device>` revokes that device on the account.

Link and unlink the host through its local API, or remove its link from the cloud account page. A trusted device with `remote:admin` can use `PATCH /api/remote/devices/{id}` and `DELETE /api/remote/devices/{id}` through a tunnel. Scopes apply to the receiving computer and revocation applies to the account. These operations use the requesting device's proof on a link-only SSH host.

## Hosted route and webhook ingress

On a linked Pro computer, run `compozy remote cloud-route enable`. Read `cloud_route.hostname`, `cloud_route.state`, and any `cloud_route.diagnosis` from local remote status. A blocked Cloudflare port 7844 shows a diagnosis while the daemon retries. The controller learns the hostname through the account list and tries it after direct and SSH. Turning the hosted route off with `compozy remote cloud-route disable` or unlinking removes the tunnel. If provisioning returns `hosted_route_taken_down`, send the operator to support@compozy.com.

A hosted browser reaches the computer at its own HTTPS hostname. Compozy Cloud starts sign-in at `GET /v1/browser/authorize`. The browser redeems the callback code through `POST /remote/v1/browser/session` with its PKCE verifier and a `DPoP` proof. The daemon exchanges the code at `POST /v1/browser/token` and issues a secure HTTP-only session cookie after the new key is trusted and any required approval is complete.

If the browser receives `device_pending_approval`, poll `GET /remote/v1/browser/approvals/{id}` with a proof from the waiting key. After approval, authorize again with that key and a fresh code. An approved account admin can decide through `POST /api/account/approvals/{id}/decision` over local HTTP or UDS. Confirm the result on `/api/account/stream` or with `GET /api/account/approvals/{id}`.

Browser requests use the session cookie and fresh proofs. For a stream, mint a single-use ticket for its path at `POST /remote/v1/stream-tickets` and send it as `remote_ticket`. Keep the terminal attach `ticket` separate. Use `GET /remote/v1/browser/peers` to discover other hosted origins. Each origin requires its own browser sign-in. Revoking the browser device ends its sessions and streams across all its origins.

The hosted route supplies the advertised HTTPS base for webhook triggers. For a self-managed HTTPS endpoint, set `remote.ingress.public_url` and forward to the tunnel listener. Keep the displayed trigger path and sign deliveries with its configured secret. Use local HTTP or UDS `POST /api/ingress/bindings` with `subject_kind: "webhook_trigger"`, `subject_id: "<trigger-id>"`, and `confirmed: true` after the operator confirms the current URL. Confirm again when the base changes. Remove a binding with `DELETE /api/ingress/bindings/webhook_trigger/{subject_id}`.

## Upgrading from Remote Gateway

In 0.8.0 the daemon migrates app tokens, allowed browser origins, and webhook bindings. First boot
removes the retired Gateway config and connection profile credentials. Set up each remote computer
again. The removed `compozy gateway`, `compozy connect`, `compozy pair`, and `compozy device`
commands exit 2 with a pointer to `compozy computer` until 0.9.0.

On the local operator API or UDS, `GET /api/migration-notices` lists current upgrade notices.
`POST /api/migration-notices/{key}/dismiss` dismisses a notice after the operator has read it.
Review migrated webhook triggers whose public address changed before reusing their delivery URLs.
[Upgrade guide](https://compozy.com/docs/migration#remote-computers-0-8-0)

Inspect outstanding notices with `GET /api/migration-notices` over local HTTP or UDS. Use
`POST /api/migration-notices/gateway_removed/dismiss` when the operator dismisses the notice.
