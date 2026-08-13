# Contributing to CompozyOS

Thanks for helping build CompozyOS. This guide covers the basics — open an issue when something here is unclear.

## Before you build anything big

For new features, open an issue describing what you want to build **before** writing code. CompozyOS is in early alpha and the design moves fast — agreeing on direction first protects your time as much as ours. Bug fixes and small improvements can go straight to a pull request.

## Dev setup

You need Go 1.26.4+ (see `go.mod`) and Bun 1.3.4 (pinned in `.bun-version`).

```bash
bun install     # install JS workspace dependencies
make build      # compile the compozy binary
make test       # Go unit tests (race detector on)
make web-dev    # run the web UI locally
```

## Quality gates

- `make gate` — runs the checks affected by your diff. Run it before every push.
- `make gate-full` — full monorepo verification. Run it before requesting review on larger changes.
- Lint is zero-tolerance: `make lint` (Go) and `bun run lint` (JS/TS) must pass with no warnings.
- A pre-commit hook formats staged files for you (oxfmt for JS/TS/YAML/Markdown, `make fmt` for Go).

## Commit messages

Format: `<type>: <description>` — type is one of `feat`, `fix`, `refactor`, `perf`, `docs`, `test`, `build`, `ci`. Don't use `chore` or `style`. A commit hook checks the format, and scopes are rejected — write `fix: …`, never `fix(web): …`.

```
fix: drop stale session sockets on daemon restart
docs: document workspace scoping for native tools
```

## Pull requests

- Keep each pull request focused on one change.
- Fill in the pull request template — especially how you verified the change.
- Behavior changes need tests. Changes to public behavior usually also need a docs update in `packages/site`.

## Reporting bugs

Use the bug report form. `compozy version` prints the version line, and `compozy support bundle --yes` produces a secret-redacted diagnostics archive you can attach. Never paste unredacted logs or API keys.

## Security

Report vulnerabilities privately — see [SECURITY.md](../SECURITY.md).
