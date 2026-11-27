# GH-1845 PR A — extension inputs for subprocesses and setup

An isolated `eng-qa-bootstrap` lab (targeted profile; CLI, API, Web) walked the changed scenarios for
[#1845](https://github.com/compozy/compozy/issues/1845) PR A on integration head `f9f47e42b`.

- **Build:** `go build` of `./cmd/compozy` from `f9f47e42b`, stamped `-X version.Version=v0.7.9`.
  `git describe` yields `v0.7.8-19-gf9f47e42b`, which normalizes to `0.7.8` and is below the
  `0.7.9` subprocess-inputs floor. Web: `vite` dev server from the same checkout proxying the lab
  daemon. The embedded UI comes from the published `compozy-web-assets` module, not this checkout.
- **Lab:** fresh home `/tmp/compozyqa-d53dc9da6054/runtime`, HTTP 58843, its own UDS, Web 58844,
  `[automation] enabled = false`, no provider. Every lab CLI call went through a wrapper that unsets
  all `COMPOZY_*` variables. The daemon ran with `REGION=eu-daemon-region` and a canary `ACME_TOKEN`
  in its own environment.
- **Fixtures** (`docs/qa/evidence/2026-11-22-gh-1845-pr-a/fixture/`): `acme-tool` declares
  `api_token` (secret, required, `ACME_TOKEN`), `project` (identifier, required, default),
  `region` (string, optional, `REGION`, also read by stdio MCP server `acme`), `workspace_ref`
  (url_query input feeding the OAuth HTTP server `acme-cloud`), and `min_compozy_version = "0.7.9"`.
  `acme-add` declares one subprocess-only secret, one MCP secret and one MCP boolean. The
  subprocess, built with the Go SDK, records only whether `ACME_TOKEN`/`REGION` are present.
  `validate` with `min_compozy_version = "0.7.8"` fails with the subprocess-inputs floor message.
- **Screenshots:** local under `docs/qa/evidence/2026-11-22-gh-1845-pr-a/screenshots/`. The R2
  upload was blocked because `wrangler` is not authenticated and no Cloudflare token is present.
- **Teardown:** manifest `TEARDOWN_COMMAND`, `teardown.json` `clean: true`, no survivors.

| Scenario | Result | Observation |
| --- | --- | --- |
| ET-ext-secrets-binding | Fail | Steps 2–4 pass. Step 1: the hidden TTY prompt, retry and install work, and no secret appears in the transcript. The install-time enable then fails with a masked 500 (BUG-20261122-install-enable-500). |
| ET-extension-setup-inputs | Fail | Steps 1, 2, 4 and 5 pass. Step 3 is unreachable as written. Step 6 passes for the reachable cells. Step 7 has no agent leg. BUG-20261122-unset-input-binding-resurrected found. |
| ET-web-marketplace-installed-management | Pass | The 2026-11-22 Setup walk passed: notice, Set up, Actions → Setup…, Missing/Set/daemon_env labels, still-needs-setup, field error, saved/restarted toasts, sign-in hint, no prefill. The dev-overlay hide was not walked. |
| ET-web-extension-union-install | Pass | Inputs part only: the Add form rendered the subprocess-only secret and the MCP inputs, masked secrets, showed the boolean switch and gated Install. The install succeeded and showed no Needs setup. |

## Session debrief — Bruno, CLI

- Install refusal with stdin redirected exits 2 with `extension_inputs_required` naming
  `api_token` and the `--input`/`--input-file` guidance. `-o json` carries `input_definitions`.
  Nothing is installed. Ctrl-C at the prompt prints `extension input prompt aborted; nothing was
  written`, exits 1 and installs nothing.
- `setup -o json` exits 0 with `region` `daemon_env`, `missing: []` and `mcp_auth_servers:
  ["acme-cloud"]`, and shows no values or Vault refs. The subprocess got `ACME_TOKEN` but not
  `REGION`, although the daemon exports `REGION`. So the daemon environment is not copied.
- The default TTY setup prompted only `region`. Enter printed the exact daemon-environment line and
  wrote nothing. The `Sign in: compozy mcp auth login acme-cloud` line appears only when an input
  feeds the OAuth server, as `extensionSetupAuthServers` implements.
- Step 3: a package update that leaves a required stored value invalid is refused at install
  preflight (`extension_inputs_required`, `invalid: ["project"]`), with or without `--enable` and
  while disabled. So setup never sees a required `invalid` input after an update. With `project`
  made optional, the update succeeded and setup reported `invalid`. The default mode does not
  prompt an optional invalid input, which matches the docs. `--all` repaired it with `Updated 1
  input of acme-tool. Restarted acme-tool.`
- Step 4: `--all` shows `(secret, set — Enter keeps it)`. Enter on every input changes nothing and
  restarts nothing. The default rerun prompts nothing only once `region` is bound. While `region`
  is `daemon_env`, the default mode prompts it by design, so the scenario's "no prompt" assumes a
  bound `region`.
- Step 5: `--input project=platform-2` with stdin redirected writes and restarts, exit 0. After
  `secrets unset --env ACME_TOKEN`, the redirected setup lists the inputs, names `api_token` and
  exits 2.
- Step 6: `setup --workspace ws-a` against the global installation fails before any prompt and
  names `--scope global`. `--profile other --input` writes only that profile's cell, and default
  is unchanged with no restart. Exit 2 because `api_token` is missing in `other`. A local-path
  package cannot hold a global and a workspace installation at once (`extension_exists`), so the
  workspace write ran after the global one was removed. ws-b stayed empty.
- Step 7: no provider exists in the lab, so the agent-identity rules
  (`extension_input_overwrite_denied`, refusal of global writes) were not exercised. As the
  operator, `compozy tool invoke compozy__extensions_setup --workspace ws-a` read state and wrote
  `region` (`changed: ["region"]`, `reloaded: true`). It also overwrote the already-set `project`,
  which operator authority permits.
- The secrets interop passes globally and in workspace scope. `secrets set` takes hidden input,
  the input reads `set` afterwards, and after a restart the subprocess receives `ACME_TOKEN`.

## Session debrief — Bruno, Web

The ws-b workspace installation had `api_token` missing and `region` `daemon_env`.
The rail showed "Needs setup: 1 required input" with Set up (01). The dialog came from
`GET .../inputs?profile=default&workspace=…&scope=workspace` and labelled the inputs Missing, Set,
and Missing plus the daemon-environment note. `acme-cloud` was listed under "These servers also
need you to sign in." with a **Sign in** action (02). Saving an empty form sent
`PUT {"inputs":{}}`. The dialog stayed open with "Still needs setup: api_token", toasts read "Setup
saved" and that region still uses the daemon environment, and `region` stayed `daemon_env` (03).
A bad identifier shows an inline field error and disables Save (04). The same value sent straight
to HTTP returned 422 `extension_input_invalid` with `input_id: "workspace_ref"`. Filling the token
closed the dialog with "Setup saved" and "Restarted acme-tool". The notice disappeared, and the
subprocess received the token (05). Actions offers Setup… above Provenance (06). The reopened
dialog showed every password field empty (07), and spec-cycle, which has no inputs, offers no
Setup…. The Add form for `acme-add` rendered both secrets masked and the boolean as an optional
switch, and Install stayed disabled until both required secrets were filled. Consent ran, the
install succeeded, and Installed showed no notice (08, 09).

## Paper cuts

| Where | Felt | Sharpness |
| --- | --- | --- |
| Extension log redaction | `KEY=present` and `token:present` print as `[REDACTED]`, so a presence-only log cannot show whether a variable is set | Low |
| Detail Setup card | `ACME_PROJECT` satisfied by a manifest default reads "From your system" | Low |
| Detail header | Reads "Blocked" while Manage says Running after setup saved | Low |
| TTY setup of an invalid boolean | Prompt shows `[y/N]` without saying the stored value is invalid | Low |
| `extension validate` error | The table column pads to the full error path width | Cosmetic |
| Daemon stop | Lab daemon survived graceful stop; teardown escalated to signals | Low |

## Not verified

- Step 7 agent identity, because the lab has no provider.
- Concurrent global plus workspace instances for one local-path package (refused by design).
- Dev-overlay hiding of Setup….
- Values in SSE and transcripts. CLI output, extension logs and the daemon log were grepped for
  every entered secret and the daemon canary, with no match.
