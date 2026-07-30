package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/cli/docpost"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

const (
	docDocKey = "doc"
)

const defaultCLIDocsDir = "packages/site/content/docs/cli"

func newDocCommand() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:    docDocKey,
		Short:  "Generate CLI reference documentation",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := cmd.Root()

			tmpDir, err := os.MkdirTemp("", "compozy-cli-docs-*")
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

			if err := docpost.Process(cmd.Context(), tmpDir, absOutput, docpost.Options{
				OutputProfiles: docOutputProfiles(root),
			}); err != nil {
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

func docOutputProfiles(root *cobra.Command) map[string]docpost.OutputProfile {
	profiles := make(map[string]docpost.OutputProfile)
	if root == nil {
		return profiles
	}
	root.InitDefaultCompletionCmd()

	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		if command == nil {
			return
		}

		profile := docpost.OutputProfileResult
		if command == root || (command.Run == nil && command.RunE == nil) {
			profile = docpost.OutputProfileHelp
		}
		switch command.CommandPath() {
		case "compozy open":
			profile = docpost.OutputProfileNoOutput
		case "compozy mcp serve":
			profile = docpost.OutputProfileProtocol
		case "compozy layout watch":
			profile = docpost.OutputProfileHumanJSONL
		}
		if strings.HasPrefix(command.CommandPath(), "compozy completion ") {
			profile = docpost.OutputProfileRaw
		}
		profiles[command.CommandPath()] = profile

		for _, child := range command.Commands() {
			visit(child)
		}
	}
	visit(root)
	return profiles
}
