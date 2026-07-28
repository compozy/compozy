## 0.3.0 - 2026-07-28

### 🎉 Features

- Introducing CompozyOS beta
- **BREAKING:** introducing CompozyOS beta

### 🐛 Bug Fixes

- _(cli)_ Reap leaked test daemons and artifacts (#253)
- Archive without tasks
- Resolve inherited cross-runtime models against runtime defaults (#259)

### 📚 Documentation

- Update readme

### 📦 Build System

- Auto-publish beta releases on release PR merge
- Push release branch updates with RELEASE_TOKEN
- Publish site changelog receipts from the release job
- Delete not need things
- Fix tests
- Align release body heading with the published version

### 🔧 CI/CD

- Fix tests

### Release Notes

#### Breaking Changes

##### The OS Release

Compozy v0.3 is a new operating system boundary for agent work. Sessions, tasks, loops, memory,
permissions, automation, the OS shell, and Compozy Network now share one daemon-owned state model.
People can start and inspect that work from the web, CLI, HTTP/SSE, or UDS. Agents can operate the
same runtime through structured tools and extension contracts.

This is a breaking beta. The command, package, environment, storage, API, and tool namespaces move
to Compozy, and several v0.2 surfaces have deliberate replacements or removals. Follow the
[v0.3 migration guide](https://compozy.com/runtime/migration/) before replacing an existing install.
The maintained v0.2 line and its collateral remain on `legacy/v0.2`.

Install the beta through the verified hosted installer, `@compozy/cli@beta`, or the explicit
`github.com/compozy/compozy@v0.3.0-beta.1` Go version. The beta channel may change before v0.3.0
stable; production rollouts should pin the version and review each prerelease.

The repository was already MIT licensed. v0.3 corrects stale BSL-1.1 text in distribution metadata;
it does not relicense the code.
