# notes-commands

A Go extension that keeps short workspace notes and presents its tools as operator commands.

It demonstrates the full [contributed-command](https://compozy.com/runtime/core/extensions/commands)
surface with nothing but the public Go SDK:

- a flat leaf, `add`
- a declared presentation group, `list`
- a nested leaf under it, `list/recent`
- projected flags across every accepted schema shape: a required string, a repeatable string array,
  a bounded integer with a default, and a string enum

## Build and link

```bash
compozy extension dev .
```

`dev` compiles the source, runs the binary in describe mode, generates
`dist/gen-<hash>/extension.toml`, and links that generation to the current workspace. No trust policy
and no consent prompt: it is your code.

## Use it

```bash
compozy extension commands notes
compozy extension exec notes --cmd add --text "ship the beta" --tag release --tag beta
compozy extension exec notes --cmd list/recent --limit 3 --format markdown
compozy extension exec notes --cmd list/recent --help
```

`--cmd list` refuses with the group's leaves, because a group is a presentation node and never
executable.

Every `exec` performs exactly one `POST /api/tools/ext__notes__<handler>/invoke`, so the same tool is
reachable by agents directly:

```bash
compozy tool invoke ext__notes__list_recent --workspace . --input '{"limit":3}'
```

## Notes stay in memory

The store is a process-local slice. Restarting the extension — including every `reload` — starts
empty. That keeps the example about the command surface rather than about persistence.

## Copy it out

The included `go.mod` requires the published SDK version and uses a checkout-only local replacement.
After copying this directory out, run:

```bash
go mod edit -dropreplace=github.com/compozy/compozy/sdk/go
go mod tidy
```
