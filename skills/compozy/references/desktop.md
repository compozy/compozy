# Desktop app

Use the `compozy app` commands to operate the desktop app on the local machine. This surface has no
native-tool equivalent.

## Commands

Read current state before acting:

```bash
compozy app status -o json
```

The preserved app verbs are `open`, `status`, `retry`, and `diagnose`. Host updates are not
an app subcommand.

Open or focus the app. The optional argument is an absolute product path such as `/workspaces`, not
a filesystem path:

```bash
compozy app open
compozy app open /workspaces
```

Check, apply, or cancel runtime and app updates through the single host update command:

```bash
compozy update --check -o json
compozy update -o json
compozy update --cancel -o json
```

Check and apply results always contain `runtime` and include `app` only when the desktop app is
installed. Cancel returns `status`, `operation_id`, `message`, and an optional `holder`. A managed
runtime returns its exact package-manager recommendation without changing the binary. Apply and
cancel are also available through `POST /api/settings/update/apply` and
`POST /api/settings/update/cancel` over HTTP or UDS; read live state with `GET /api/settings/update`.
Treat `available`, `applying`, `staged`, `blocked`, `failed`, `updated`, and
`up-to-date` as the update statuses. A blocked result names the current holder. App status also
projects the operation ID, phase, and progress when an operation exists.

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

App updates require consent and restart the desktop process. The runtime updates first. A running
shell applies its verified app artifact; when the shell is absent, the daemon records a staged app
operation for the shell to complete after its next launch. The app can replace an app-owned runtime
after verification when durable provenance still proves ownership. For an operator-managed or
inconclusively owned runtime, report the install method and its update command; never replace the
binary through `compozy app`.

## Shell facts

The desktop app is an Electron shell over the daemon-served product UI. A packaged app includes a
verified runtime, so a clean first run can provision offline. Boot resolves in this order: attach to
a healthy daemon, start `$COMPOZY_HOME/bin/compozy`, or verify and install the bundled runtime.
Quitting the shell leaves the daemon running.

On macOS and Linux, a desktop-owned daemon refreshes only `PATH` from the operator's login shell
before starting runtime services. Failure preserves the inherited environment. Operator-managed
daemons and Windows keep their launcher environment.

The product UI is developed and release-verified against Chromium, the engine embedded by Electron.
Other browsers can open the daemon-served UI on a best-effort basis.

## Recovery

When an update reports `failed`, do not start another mutation. Run `compozy app diagnose -o json`,
preserve the reported error code and report, follow its recovery action, then run
`compozy app retry`. Confirm the result with `compozy app status -o json`.

Automatic repair is limited to disposable desktop metadata and a runtime process proven to be
desktop-owned. Never stop an operator-managed runtime or delete `compozy.db`, `config.toml`,
credentials, sessions, or the full home as a recovery step. The desktop has no native-tool
equivalent; use the local `compozy app` control surface.

After every app mutation, perform a structured status read. Do not infer success from a process,
window, or human-readable message alone.

## Page zoom

The desktop main window supports the standard page zoom shortcuts: `Command` + `+`, `Command` +
`-`, and `Command` + `0` on macOS; use `Control` instead of `Command` on Windows and Linux. These
shortcuts scale the whole product interface. The in-product Window > Zoom action has a different
purpose: it expands one CompozyOS window inside the desktop workspace.

## Native editing

Editable fields use the platform Edit menu. Cut, copy, paste, and select all use `Command` on macOS
and `Control` on Windows and Linux.
