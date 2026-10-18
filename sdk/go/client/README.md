# CompozyOS Go app client

Build applications against a running CompozyOS daemon with typed HTTP methods,
server-sent events, and streamed prompts. This package uses only the Go standard
library and ships in the existing `github.com/compozy/compozy/sdk/go` module.

> Experimental in this release: names and shapes may change in the next release without a deprecation
> window. Pin the exact version.

```go
import "github.com/compozy/compozy/sdk/go/client"

c, err := client.New(ctx)
if err != nil {
    return err
}
workspace, err := c.ResolveWorkspace(ctx, client.ResolveWorkspaceRequest{Path: "/my/project"})
if err != nil {
    return err
}
```

`New` tries an explicit `WithBaseURL`, then `COMPOZY_URL`, the daemon's unix socket,
and the TCP address in `daemon.json`. `WithHome` overrides `COMPOZY_HOME`; the
default home is `~/.compozy`. Use `WithToken` to attach an app token. Every method
accepts a context, and streams expose `Next` and `Close`. `Prompt` handles both
streamed replies and accepted input; inspect `Accepted` before reading chunks.

Generated methods retain each operation's transport constraints. HTTP-only operations require
`WithBaseURL` pointing to the daemon's HTTP address; the default socket connection returns 404 for
those routes. See the [Go tutorial's transport guidance](https://compozy.com/docs/apps/go#create-a-session-and-prompt-it)
for the methods and the OpenAPI `x-compozy-transports` metadata.

Operations with distinct successful response bodies preserve the complete JSON and expose
`StatusCode`. For an operator `InvokeTool` call, decode a 200 with `AsToolInvokeResponse` or a
202 with `AsToolErrorResponse` to retain its approval details.

Follow the [Go app tutorial](https://compozy.com/docs/apps/go) for installation,
session creation, prompts, and stream handling. See the generated
[package reference](https://pkg.go.dev/github.com/compozy/compozy/sdk/go/client)
for every operation and DTO.
