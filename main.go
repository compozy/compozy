// Package main is the root Compozy CLI entrypoint for go install github.com/compozy/compozy.
package main

import (
	"context"
	"io"
	"os"

	"github.com/compozy/compozy/internal/cli"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	return cli.ExecuteContext(ctx, args, stdout, stderr)
}
