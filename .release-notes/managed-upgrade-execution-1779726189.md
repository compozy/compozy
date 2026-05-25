---
title: `compozy upgrade` runs the right package-manager command automatically
type: fix
---

`compozy upgrade` previously only **printed** the package-manager command for managed installs (Homebrew, npm, `go install`), leaving the user to copy-paste it. It now **executes** the correct command directly, targeting the exact install layout detected for the running binary so the upgrade lands on the same install Compozy is currently running from.

### Behavior change

| Install method | Before                                                             | After                                                                                                  |
| -------------- | ------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------ |
| Homebrew       | Printed `brew upgrade compozy/compozy/compozy`                     | Runs `brew upgrade compozy/compozy/compozy` against the detected Homebrew prefix                       |
| npm (global)   | Printed `npm install -g @compozy/cli@latest`                       | Runs `npm install -g @compozy/cli@latest` with `NPM_CONFIG_PREFIX` pointing at the detected npm prefix |
| `go install`   | Printed `go install github.com/compozy/compozy/cmd/compozy@latest` | Runs the same command with `GOBIN` set to the detected install bin directory                           |
| Binary         | In-place self-update                                               | Unchanged — still self-updates                                                                         |

Direct binary installs continue to perform the in-place self-update as before. Output (stdout + stderr) from the underlying package manager streams to the user.

### Why this is a real fix, not just a UX polish

The previous "print only" flow had two failure modes that were silent for users:

1. **Multi-install systems** (e.g. Homebrew binary on `PATH` while a `~/go/bin/compozy` is also installed) — copy-pasting the printed command could upgrade the wrong install.
2. **Windows shells** — the printed command was a hint, not always copy-paste-correct for `cmd.exe` / PowerShell.

The new flow resolves the actual executable path, locates the install's package-manager prefix, and runs the upgrade command with that prefix forced via env (`NPM_CONFIG_PREFIX`, `GOBIN`) or `PATH` prefix, so the right install gets upgraded every time.

### Internals

- `installDetails` now captures the resolved executable path and environment alongside the install method (`internal/update/install.go`).
- `managedUpgradeCommandForInstall` builds a `managedUpgradeCommand{ name, args, pathPrefix, env }` per install method.
- `defaultManagedUpgradeCommand` invokes it via `exec.CommandContext(ctx, …)` with an isolated env so the upgrade does not leak host env into the spawned process.
- Windows-specific environment-key handling lands so the upgrade works under PowerShell / `cmd.exe`.

### Help text

```
$ compozy upgrade --help
…
Package-manager installs run the correct package manager command. Direct binary installs
perform an in-place self-update.
```
