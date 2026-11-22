---
id: ET-ext-secrets-binding
area: ET
title: Bind extension secrets without exposing them
persona: Bruno
journey: J-extension-kit-lifecycle
expected: A declared environment key binds to an existing Vault reference or hidden input at the exact extension instance scope, while list and every transport expose only bound key names.
entry_points: /docs/extensions/secrets, /docs/extensions/install and CLI extension secrets reference; compozy extension install (TTY input prompt); compozy extension secrets set --value-stdin|bind|list|unset [--workspace] -o json|jsonl|toon; GET|PUT|DELETE /api/extensions/:name/secrets[/env] over HTTP and UDS; compozy doctor --only extension
qa_status: fail
bug_ids: [BUG-20261122-install-enable-500, BUG-20261122-unset-input-binding-resurrected]
fix_status: pending
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-11-22-gh-1845-pr-a
last_report: docs/qa/reports/2026-11-22-gh-1845-pr-a-extension-inputs.md
overlaps: ET-019; ET-web-extension-detail; ET-extension-setup-inputs
---

QA impact 2026-08-02: new secret-binding lifecycle. Walk global and workspace instances, update
survival, stale declarations, unset/remove GC, deterministic 400 binding classes, and
plaintext/reference redaction across output, logs, events, SSE, and transcripts. No native tool may
write a secret.

QA impact 2026-11-22 (#1845, subprocess inputs): the extension subprocess now receives inputs bound
to a `requires_env` key. Status reset to `untested` until this addition is walked; the earlier walk
above still stands for bindings.

Fixture: a local extension with `requires_env = ["ACME_TOKEN"]`, one required `secret` input
`api_token` bound to `{type="env", name="ACME_TOKEN"}`, `min_compozy_version` at the subprocess-inputs
floor, and a subprocess that writes whether `ACME_TOKEN` is set (never the value) to its log.

Steps:

1. In a real terminal, `compozy extension install <path> --enable`. The CLI refuses with
   `extension_inputs_required`, prompts `API token for Acme (secret): ` with hidden input, and retries;
   the extension installs and starts. The log shows `ACME_TOKEN` present.
2. Repeat the install with stdin redirected (`</dev/null`) and with `-o json`: no prompt, the message
   names `api_token` with the `--input`/`--input-file` guidance, exit code 2, nothing installed.
3. Press Ctrl-C at the prompt: exit without a retry, nothing installed.
4. Install a second instance (`--workspace`) with `--input api_token=...`, then
   `compozy extension secrets unset <name> --env ACME_TOKEN --workspace <ws>` followed by
   `compozy extension secrets set <name> --env ACME_TOKEN --workspace <ws>`: `compozy extension setup
   <name> --workspace <ws> -o json` shows `api_token` `set`, and after restart the subprocess still
   receives `ACME_TOKEN`.

Pass: the value never appears in CLI output, logs, events, SSE or transcripts; the daemon's own
`ACME_TOKEN` environment variable never reaches the subprocess.

src: .compozy/tasks/extension-inputs-setup-deps/_tests.md

QA 2026-11-22 (#1845 subprocess inputs): steps 2–4 pass, with no value in any transcript or log
and the daemon's own environment absent from the subprocess. Step 1: the hidden prompt, retry and
install work, but the install-time enable fails with a masked 500
(BUG-20261122-install-enable-500); a later `enable` starts the extension and it receives
`ACME_TOKEN`. Details in the dated report.
