# Hello World in Go

This example creates a minimal lifecycle observer in Go that logs the final run status.

## 1. Create the project

```bash
mkdir hello-go
cd hello-go
go mod init example.com/hello-go
go get github.com/compozy/compozy/sdk/extension@v0.1.10
go mod tidy
```

## 2. Add `main.go`

```go
package main

import (
	"context"
	"fmt"
	"os"

	extension "github.com/compozy/compozy/sdk/extension"
)

func main() {
	ext := extension.New("hello-go", "0.1.0").
		OnRunPostShutdown(func(
			_ context.Context,
			_ extension.HookContext,
			payload extension.RunPostShutdownPayload,
		) error {
			fmt.Fprintf(os.Stderr, "run %s finished with %s\n", payload.RunID, payload.Summary.Status)
			return nil
		})

	if err := ext.Start(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

## 3. Add `extension.toml`

```toml
[extension]
name = "hello-go"
version = "0.1.0"
description = "Hello-world Go extension"
min_compozy_version = "0.1.10"

[subprocess]
command = "go"
args = ["run", "."]

[security]
capabilities = ["run.mutate"]

[[hooks]]
event = "run.post_shutdown"
```

## 4. Install and enable

```bash
compozy ext install --yes .
compozy ext enable hello-go
```

## 5. Run Compozy

```bash
compozy start
```

You should see the final run status on stderr when the run shuts down.

## Faster path

If you do not want to create the files manually, the scaffolder can generate the same starting point:

```bash
npx @compozy/create-extension hello-go --template lifecycle-observer --runtime go --module example.com/hello-go
```
