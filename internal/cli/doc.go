package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/agh/internal/cli/docpost"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

const (
	docDocKey = "doc"
)

const defaultCLIDocsDir = "packages/site/content/runtime/cli-reference"

func newDocCommand() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:    docDocKey,
		Short:  "Generate CLI reference documentation",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := cmd.Root()

			tmpDir, err := os.MkdirTemp("", "agh-cli-docs-*")
			if err != nil {
				return fmt.Errorf("doc: create temp dir: %w", err)
			}
			defer os.RemoveAll(tmpDir)

			if err := doc.GenMarkdownTree(root, tmpDir); err != nil {
				return fmt.Errorf("doc: generate markdown: %w", err)
			}

			absOutput, err := filepath.Abs(outputDir)
			if err != nil {
				return fmt.Errorf("doc: resolve output path: %w", err)
			}

			if err := docpost.Process(cmd.Context(), tmpDir, absOutput); err != nil {
				return fmt.Errorf("doc: post-process: %w", err)
			}

			cmd.Printf("CLI docs generated in %s\n", absOutput)
			return nil
		},
	}

	cmd.Flags().StringVar(&outputDir, "output-dir", defaultCLIDocsDir,
		"Output directory for generated CLI reference MDX files")

	return cmd
}
