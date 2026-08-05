package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/compozy/compozy/internal/marketplace"
)

const catalogDigestCommand = "digest"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if ctx == nil {
		return errors.New("compozy-catalog: context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("compozy-catalog: command canceled: %w", err)
	}
	if len(args) < 2 {
		return catalogUsageError()
	}
	switch args[0] {
	case "validate":
		if len(args) != 2 {
			return catalogUsageError()
		}
		return validateCatalogForPublication(ctx, args[1])
	case catalogDigestCommand:
		if len(args) != 2 {
			return catalogUsageError()
		}
		digest, err := marketplace.DigestFile(args[1])
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(output, digest); err != nil {
			return fmt.Errorf("write catalog digest: %w", err)
		}
		return nil
	case "package":
		if len(args) != 3 {
			return catalogUsageError()
		}
		return packageCatalogExtension(args[1], args[2])
	default:
		return fmt.Errorf("unknown compozy-catalog command %q", args[0])
	}
}

func catalogUsageError() error {
	return errors.New(
		"usage: compozy-catalog <validate DIRECTORY|digest ARTIFACT|package SOURCE_DIRECTORY OUTPUT_ARCHIVE>",
	)
}
