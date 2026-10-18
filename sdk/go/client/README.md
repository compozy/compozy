# CompozyOS Go app client

Build applications against a running CompozyOS daemon with typed HTTP methods,
server-sent events, and streamed prompts. This package uses only the Go standard
library and ships in the existing `github.com/compozy/compozy/sdk/go` module.

> Experimental in 0.4: names and shapes may change in 0.5 without a deprecation
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

Follow the [Go app tutorial](https://compozy.com/docs/apps/go) for installation,
session creation, prompts, and stream handling. See the generated
[package reference](https://pkg.go.dev/github.com/compozy/compozy/sdk/go/client)
for every operation and DTO.
