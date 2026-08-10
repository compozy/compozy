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
```

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
`compozy app diagnose -o json`, preserve the reported recovery code and paths, follow its recovery
action, then run `compozy app retry`. Confirm the result with `compozy app status -o json`.

After every app mutation, perform a structured status read. Do not infer success from a process,
window, or human-readable message alone.
