package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/compozy/agh/internal/marketplace"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	if len(args) < 2 {
		return catalogUsageError()
	}
	switch args[0] {
	case "validate":
		if len(args) != 2 {
			return catalogUsageError()
		}
		return validateCatalogForPublication(args[1])
	case "digest":
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
		return fmt.Errorf("unknown agh-catalog command %q", args[0])
	}
}

func catalogUsageError() error {
	return errors.New(
		"usage: agh-catalog <validate DIRECTORY|digest ARTIFACT|package SOURCE_DIRECTORY OUTPUT_ARCHIVE>",
	)
}
