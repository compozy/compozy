# clarify-tool

A Go tool provider that asks the session operator one bounded question while handling a tool call.

It demonstrates:

- registering a typed tool with the public Go SDK
- declaring the single permission it needs, `clarify/ask`
- `ToolRequest.AskClarification`, which blocks until the operator answers, the configured timeout
  returns the fallback sentinel, or the tool call is canceled

The daemon binds each clarification to the extension and the active session and derives workspace and
agent scope itself. Extension code supplies only the question and optional choices; it cannot select
another session or forge invocation authority.

The SDK declaration is the source of identity, permissions, schemas, and subprocess metadata.
`compozy extension build` generates the manifest.

## Build and link

```bash
compozy extension dev .
compozy tool invoke ext__clarify_tool__ask --workspace . --input '{"question":"Ship it?","choices":["yes","no"]}'
```

## Copy it out

The included `go.mod` requires the published SDK version and uses a checkout-only local replacement.
After copying this directory out, run:

```bash
go mod edit -dropreplace=github.com/compozy/compozy/sdk/go
go mod tidy
```
