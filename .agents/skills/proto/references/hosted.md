# Hosted operations

Local authoring, human review and static builds are Free. Local agent operations use a Pro
Editor seat or the account's one-time 100-operation allowance and require an online check. See
[CLI](cli.md) for the local plan rules. Hosted publishing requires a signed-in Editor and a
Pro workspace. Run `proto login`, `proto whoami`, `proto workspace list`, then
`proto workspace use <slug>`. `proto logout` removes the CLI session.

Publish exactly one target: `proto share <slug>`, `proto share --flow <id>`, or `proto share --all`.
Optional repeated `--viewer`, `--commenter`, and `--approver` flags take email addresses.
During `proto login`, finish Google/GitHub or email sign-in and accept the terms in the browser
before approving the device. The browser returns to the same device code after acceptance. If
polling reports `legal_required`, the CLI names that browser page and asks you to retry login
once acceptance is complete; no credential is saved for the refused attempt.

Use `--app <dir>` to select the app and `--json` for structured output. On a Free workspace,
`proto share` answers `plan_required` with the workspace's upgrade URL and changes nothing.
Snapshots persist after the command exits.

On a busy response (HTTP 429 or 503), the CLI makes up to three attempts at the final publish
request. It honors `Retry-After` with jitter and starts retries within 60 seconds. An attempt already
in progress can finish after that window. Upload creation and blob PUTs are sent once.

Publish with `--note "Ready for review"` to attach a 1–280 character version note. Proto captures
up to 60 light-theme 640×400 WebP previews in registry order and uses the first as the cover.
Use `--no-captures` to skip screenshots. If Playwright is missing, publishing continues and
prints the Playwright package install command on stderr. If Chromium is missing, it prints
the browser install command. Both use the detected package manager. Capture allows 15 seconds
per state and 120 seconds overall. Failed and timed-out captures are skipped with one warning.
Publish JSON adds `captures` (the stored capture count) and `note` (the version note or null).
An unchanged snapshot keeps its stored thumbnails and note.

Use `proto share list`, `proto share invite <slug> --commenter <email>` and
`proto share revoke <slug>`. Publish again to update a target's snapshot. People review through
the hosted link; invitees sign in with their invited email. Approvals happen in the browser.

Store commands use `--remote --share <slug>`, for example:

```sh
proto comments list --remote --share checkout --json
proto comments reply checkout-1 --note "Updated the empty state." --remote --share checkout --json
proto review status --remote --share checkout --json
```

The local stdio server `proto mcp --app <dir>` accepts `remote: true` and `share` on store tools.
Each call through that server still needs local agent access. CLI store commands using
`--remote --share <slug>` use the hosted share's access rules.
For hosted access without a checkout, configure an OAuth-capable Streamable HTTP MCP client with
`https://mcp.protohq.design/mcp`. Complete browser authorization for the intended workspace.
Remote tools operate within that grant; CLI credentials are not remote MCP OAuth grants.
Manage grants and CLI devices in Settings → CLI and agent access. Revoking a grant ends access.
Agents never approve reviews. Remote MCP provides share discovery, inbox, comment replies,
resolve/reopen, attachment access and review status; it does not publish or manage billing.

`PROTOHQ_TOKEN` supplies a CLI credential, `PROTOHQ_WORKSPACE` selects the workspace, and
`PROTOHQ_API_URL` accepts a loopback override for local development or exactly
`https://staging-api.protohq.design` to target staging (links then use
`https://staging-app.protohq.design`); any other value fails with `api_url_refused`. Never put tokens in
project config, logs or documentation. See [errors](errors.md) for exact registry codes.

Read [remote MCP](remote-mcp.md) for OAuth consent identity, the nine tools, quotas and grant last use.

## Web collaboration

Workspace home lists hosted shares, review/lifecycle status and plan controls. The inbox
filters threads by status, share and participation, with pagination and links into the hosted shell.
Owners/admins manage invitations, roles and Editor seats on Members. Workspace invitations load
`GET /v1/invitations/{id}` and require explicit `POST /v1/invitations/{id}/accept` after signing in
with the invited email. Agents must not treat invitation previews or pending billing changes as
completed membership changes.
