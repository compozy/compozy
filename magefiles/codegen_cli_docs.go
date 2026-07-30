//go:build mage

package main

import (
	"context"
	"fmt"
)

const (
	cliDocsOutputDir = "packages/site/content/docs/cli"
	cliDocsCheckPath = "scripts/check-cli-docs.sh"
)

// CLIDocs regenerates the CLI reference from the live Cobra command tree.
func CLIDocs() error {
	ctx := context.Background()
	if err := runCommandInDir(
		ctx,
		".",
		"go",
		"run",
		"./cmd/compozy",
		"doc",
		"--output-dir",
		cliDocsOutputDir,
	); err != nil {
		return fmt.Errorf("generate CLI docs: %w", err)
	}
	if err := runCommandInDir(ctx, ".", "bunx", "oxfmt", cliDocsOutputDir); err != nil {
		return fmt.Errorf("format CLI docs: %w", err)
	}
	return nil
}

// CLIDocsCheck verifies that the committed CLI reference matches Cobra.
func CLIDocsCheck() error {
	if err := runCommandInDir(context.Background(), ".", "bash", cliDocsCheckPath); err != nil {
		return fmt.Errorf("check CLI docs: %w", err)
	}
	return nil
}
