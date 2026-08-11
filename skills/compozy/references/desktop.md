# Desktop app

Use the `compozy app` commands to operate the desktop app on the local machine. This surface has no
native-tool equivalent.

## Commands

Read current state before acting:

```bash
compozy app status -o json
```

Open or focus the app. The optional argument is an absolute product path such as `/workspaces`, not
a filesystem path:

```bash
compozy app open
compozy app open /workspaces
```

Check or apply updates:

```bash
compozy app update --check -o json
compozy app update --apply app
compozy app update --apply runtime
```

Retry the current operation or request structured diagnostics:

```bash
compozy app retry
compozy app diagnose -o json
compozy app diagnose --bundle --yes -o json
compozy app diagnose --bundle --yes --bundle-output ./desktop-diagnostics.tar.gz -o json
```

`compozy app diagnose -o json` returns the safe `DiagnosticReport`, including the boot ID and
phase, versions, ownership, current safe error, and any previous crash. It omits raw paths, log
contents, and secrets. If the app is not running and the control socket is absent, it reads the
latest persisted report, so it does not require a healthy daemon. A present but unresponsive socket
returns its control error.

`--bundle` writes a local archive only when `--yes` explicitly confirms the write. The default
location is `$COMPOZY_HOME/support-bundles/`; `--bundle-output` selects a new `.gz` file path and
refuses existing files or symbolic links. The archive has `manifest.json` containing the redacted
report and may include bounded, redacted tails from the current boot's `desktop.log` and
`desktop-bootstrap.jsonl`. It never includes `compozy.log`, raw logs, databases, configuration,
credentials, sessions, or transcripts. It is never uploaded automatically and is separate from
`compozy support bundle`, which is a daemon-owned runtime-support operation.

## Ownership

Treat attachment and ownership separately. Attaching to an existing daemon never transfers
ownership. Quitting the app never stops any runtime, including one the app started or provisioned.
Use the runtime's own control surface for an intentional stop.

App updates require consent and restart the desktop process. The app can replace an app-owned
runtime after verification when durable provenance still proves ownership. For an operator-managed
or inconclusively owned runtime, report the install method and its
update command; never replace the binary through `compozy app`.

## Recovery

When status reports `recovery_required`, do not start another mutation. Run
`compozy app diagnose -o json`, preserve the reported recovery code and report, follow its recovery
action, then run `compozy app retry`. Confirm the result with `compozy app status -o json`.

Automatic repair is limited to disposable desktop metadata and a runtime process proven to be
desktop-owned. Never stop an operator-managed runtime or delete `compozy.db`, `config.toml`,
credentials, sessions, or the full home as a recovery step. The desktop has no native-tool
equivalent; use the local `compozy app` control surface.

After every app mutation, perform a structured status read. Do not infer success from a process,
window, or human-readable message alone.
