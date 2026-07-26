<div align="center">
  <img src="packages/site/public/icon-512.png" alt="Compozy" width="96" height="96">
  <h1>CompozyOS</h1>
  <p><strong>A local-first operating system for agent work.</strong></p>
  <p>
    <a href="https://github.com/compozy/compozy/actions/workflows/ci.yml">
      <img src="https://github.com/compozy/compozy/actions/workflows/ci.yml/badge.svg" alt="CI">
    </a>
    <a href="https://github.com/compozy/compozy/releases">
      <img src="https://img.shields.io/github/v/release/compozy/compozy?include_prereleases" alt="Release">
    </a>
    <a href="https://goreportcard.com/report/github.com/compozy/compozy">
      <img src="https://goreportcard.com/badge/github.com/compozy/compozy" alt="Go Report Card">
    </a>
    <a href="LICENSE">
      <img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT">
    </a>
  </p>
</div>

Give agents work that keeps going after a terminal tab closes. CompozyOS connects durable sessions, tasks, loops, memory, permissions, automation, and an OS-style web shell through one local daemon. People can see and steer the work; agents can operate the same state through structured controls.

The complete documentation lives at [compozy.com](https://compozy.com).

<div align="center">
  <img src="docs/design/screen.png" alt="CompozyOS workspace" width="100%">
</div>

## Highlights

- **One connected system.** Sessions, work, memory, permissions, automation, tools, and the web shell share daemon-owned state instead of behaving like separate products.
- **Local-first durability.** One Go binary and SQLite-backed daemon keep agent work resumable and inspectable long after the terminal closes.
- **Shared control.** Web, CLI, HTTP/SSE, UDS, MCP, and native tools expose the same runtime state to people and agents.
- **Built to be built on.** Extensions, hooks, skills, capabilities, bridges, SDKs, and tools plug into public runtime contracts.
- **Bounded autonomy.** Task runs, claim tokens, leases, memory scopes, and safe spawn keep multi-agent work observable and recoverable.
- **Compozy Network.** Sessions can discover peers, exchange typed envelopes on `compozy-network/v0`, share capabilities, and close work with receipts.

## Install

```bash
curl -fsSL https://compozy.com/install.sh | sh
```

Homebrew:

```bash
brew install compozy/compozy/agh
```

npm:

```bash
npm install -g @compozy/agh
```

Go:

```bash
go install github.com/compozy/compozy@latest
```

The full [Installation guide](https://compozy.com/runtime/core/getting-started/installation) covers the verified binary installer, Linux packages, and source builds.

## Quick start

```bash
compozy install
compozy daemon start
compozy workspace add "$PWD" --name current
compozy session new --workspace current --agent general
```

See the [Quick Start](https://compozy.com/runtime/core/getting-started/quick-start) for the full walkthrough.

## Documentation

- [Runtime overview](https://compozy.com/runtime)
- [Installation](https://compozy.com/runtime/core/getting-started/installation)
- [Quick Start](https://compozy.com/runtime/core/getting-started/quick-start)
- [CLI reference](https://compozy.com/runtime/cli-reference)
- [Extensions](https://compozy.com/runtime/core/extensions)
- [Compozy Network protocol](https://compozy.com/protocol)
- [GitHub releases](https://github.com/compozy/compozy/releases)

## Development

CompozyOS is a Go and Bun monorepo. Start the daemon with automatic Go rebuilds and the web UI with Vite HMR:

```bash
make dev
```

The first successful build stops any daemon using the active `COMPOZY_HOME` and takes over its lifecycle. A failed Go rebuild keeps the last successful daemon running; the next successful build replaces it. Vite uses the first available port starting at `3000`, and the daemon's web routes redirect to that live UI while API traffic stays on the daemon. Set `COMPOZY_WEB_PORT` to require a specific web port. Press `Ctrl-C` to stop the owned daemon and both development processes, or use `make dev-daemon` when you only need the backend.

Run the full verification gate before sending changes:

```bash
make verify
```

## Contributing

Contributions are welcome. Open an issue or pull request, and run `make verify` before sending changes.

## Contributors

Thanks to everyone who has contributed to Compozy.

<a href="https://github.com/compozy/compozy/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=compozy/compozy" alt="Contributors" />
</a>

## License

Compozy is distributed under the [MIT License](LICENSE).

## Star history

<a href="https://www.star-history.com/?repos=compozy%2Fcompozy&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=compozy/compozy&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=compozy/compozy&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=compozy/compozy&type=date&legend=top-left" />
 </picture>
</a>
