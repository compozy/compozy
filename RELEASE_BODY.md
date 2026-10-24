## 0.4.0 - 2026-10-24

### Bug Fixes

- Validate bridge desktop smoke against its update feed
- Refresh Claude usage after account ownership changes (#931)
- Revalidate configuration before reusing workspace snapshots
- Format workspace snapshot configuration checks
- Align stable release dry-run verification
- Resume interrupted sessions after daemon restart (#942)
- Isolate window surfaces from geometry updates (#943)
- Recover stops after runtime identity release (#948)
- Reveal native subagent work from session lists (#949)
- Keep async subagent navigation outside compiler scope
- Preserve recovered stop reasons and native reveal state
- Preserve current branch and destination in session board actions
- Reveal settled native subagents by card identity
- Wrap current branch queries within source line limits
- Stop waiting on completed subagent delegations
- Collect delivery facts independently of PR display cache

### Features

- Add managed worktree teardown commands (#937)
- Link session pull requests, add waiting badge and sessions Kanban (#944)

### Performance Improvements

- Share hosted MCP projections across equivalent peers (#938)
- Make session inventory reads cheap and side-effect free (#940)
- Reduce idle skill watcher and workspace resolver scans (#936)

### Refactoring

- Group session catalog exports with board surface

### Testing

- Align restart fixtures with explicit restart admission
- Include pull request columns in CLI round-trip contract
- Attach successor approval to the named wake turn
- Separate migration verification from upgrade budget

### Release Notes

#### Highlights

##### CompozyOS is stable

v0.4.0 is the first stable CompozyOS release. It closes the v0.3.0 beta line, and every release from here on is a stable release.

- **On a v0.3.0 beta:** update in place from the app's Update action or with `compozy update`. No reinstall, and your sessions, projects, and settings carry over.
- **On v0.2:** v0.2 has reached end of life and receives no further releases. Homebrew and npm now install v0.4.0, which starts from clean runtime state. Read the [migration guide](https://compozy.com/docs/migration) before upgrading.
- **New installs:** `brew install compozy/compozy/compozy`, `npm install -g @compozy/cli`, or the verified installer at `curl -fsSL https://compozy.com/install.sh | sh`.

These notes cover the changes since the last beta. The full beta trail lives in the [GitHub releases](https://github.com/compozy/compozy/releases) and the [changelog](https://compozy.com/changelog).
