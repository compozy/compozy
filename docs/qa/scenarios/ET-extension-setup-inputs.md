---
id: ET-extension-setup-inputs
area: ET
title: Complete an installed extension's inputs from the CLI and an agent without overwriting set values
persona: Bruno
journey: J-extension-kit-lifecycle
expected: compozy extension setup prompts only for missing, invalid and daemon-environment inputs (or every input with --all), writes them to the selected instance, restarts it when enabled, and points to mcp auth login for OAuth servers; an agent through compozy__extensions_setup fills only missing, invalid or daemon_env inputs and is refused with extension_input_overwrite_denied for a set one.
entry_points: /docs/extensions/install#complete-inputs-after-install and CLI extension setup reference; compozy extension setup <name> [--scope] [--workspace] [--profile] [--all] [--input|--input-file] [-o json|toon]; GET|PUT /api/extensions/:name/inputs over HTTP and UDS; compozy__extensions_setup
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-ext-secrets-binding; ET-web-marketplace-installed-management
---

Covers US-005, US-006 and US-007 (#1845).

Lab: `eng-qa-bootstrap` isolated lab (unique `COMPOZY_HOME`, ports and socket) built from the
integrated `feat/extension-inputs-setup-deps` head, with one real provider session for the agent step.
Capture every command with timestamp, output and exit code.

Fixture: a local extension `acme-tool` with
- `api_token` (`secret`, required, bound to `ACME_TOKEN` in `requires_env`);
- `project` (`identifier`, required, default `default-project`);
- `region` (`string`, optional) referenced by a stdio MCP server `acme` and by the subprocess;
- a second packaged HTTP MCP server with `auth.method = "oauth"` whose URL template uses `project`.

Steps:

1. Install with `--input api_token=... --enable`, with `region` unset but `REGION` exported in the
   daemon environment. `compozy extension setup acme-tool -o json` exits 0, shows `region` as
   `daemon_env`, `missing: []`, and `mcp_auth_servers` with the OAuth server; no value or Vault ref
   appears.
2. In a real terminal, `compozy extension setup acme-tool`: it prompts only for `region`; Enter prints
   `Acme region: still using the daemon environment; the extension process does not receive it.`;
   nothing is written and the output ends with `Sign in: compozy mcp auth login <server>`.
3. Change the `project` input type in a package update so the stored value no longer fits. Setup
   prompts for `project` (state `invalid`); entering a valid value prints `Updated 1 input of
   acme-tool. Restarted acme-tool.`
4. `compozy extension setup acme-tool --all`: every input is prompted; `api_token` shows
   `(secret, set — Enter keeps it)`; Enter on all keeps everything and changes nothing. Rerun the
   default mode: no prompt, `All required inputs of acme-tool are set.`, no restart.
5. `compozy extension setup acme-tool --input project=platform-2` with stdin redirected: writes
   without prompting and exits 0. Remove the `api_token` binding with `secrets unset` and rerun
   setup with stdin redirected: it lists the inputs, names `api_token`, and exits 2.
6. Repeat step 5's write on a `--workspace` instance and with `--profile <other>`; the global
   instance and the other profile are unchanged.
7. From the agent session, call `compozy__extensions_setup {name: "acme-tool"}` on the global
   installation: the read succeeds, and a write is refused as a scope error with nothing written.
   Install `acme-tool` in the agent's workspace, then read, write a value for the `daemon_env` input
   `region` (accepted, now `set`; only that workspace's runtime restarts), then write `project`
   (already `set`): refused with `extension_input_overwrite_denied` listing `project`, nothing
   written. The global instance and other workspaces stay unchanged.

Pass: values never appear in output, logs, events, SSE or transcripts; the installed version,
attachments and enablement never change.

src: .compozy/tasks/extension-inputs-setup-deps/_tests.md
