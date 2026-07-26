package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/fileutil"
	"github.com/spf13/cobra"
)

const (
	bridgeManifestDefaultFile = "slack-manifest.json"
	bridgeManifestEpilogue    = "paste into api.slack.com → From an app manifest"
	bridgeManifestPlatform    = "slack"
)

func newBridgeManifestCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Generate provider setup artifacts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newBridgeSlackManifestCommand(deps))
	return cmd
}

func newBridgeSlackManifestCommand(deps commandDeps) *cobra.Command {
	var instanceID string
	var writeManifest bool
	var outputPath string

	cmd := &cobra.Command{
		Use:   bridgeManifestPlatform,
		Short: "Generate a Slack app manifest for one bridge instance",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			trimmedID := strings.TrimSpace(instanceID)
			if trimmedID == "" {
				return errors.New("cli: --instance is required")
			}
			if cmd.Flags().Changed("out") && !writeManifest {
				return errors.New("cli: --out requires --write")
			}
			if cmd.Flags().Changed("out") && strings.TrimSpace(outputPath) == "" {
				return errors.New("cli: --out cannot be blank")
			}

			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			manifest, err := client.SlackBridgeManifest(cmd.Context(), trimmedID)
			if err != nil {
				return err
			}
			if !writeManifest {
				return writeJSON(cmd, manifest)
			}

			target, err := slackManifestTargetPath(deps, outputPath)
			if err != nil {
				return err
			}
			payload, err := encodeSlackManifest(manifest)
			if err != nil {
				return err
			}
			if err := fileutil.AtomicWriteFile(target, payload, 0o644); err != nil {
				return fmt.Errorf("cli: write Slack manifest %q: %w", target, err)
			}
			if _, err := fmt.Fprintf(
				cmd.ErrOrStderr(),
				"Slack manifest written to: %s\n%s\n",
				target,
				bridgeManifestEpilogue,
			); err != nil {
				return fmt.Errorf("cli: write Slack manifest next steps: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&instanceID, "instance", "", "Bridge instance id")
	cmd.Flags().BoolVar(&writeManifest, "write", false, "Write the manifest to a file")
	cmd.Flags().StringVar(&outputPath, "out", "", "Manifest output path (requires --write)")
	return cmd
}

func slackManifestTargetPath(deps commandDeps, outputPath string) (string, error) {
	if trimmed := strings.TrimSpace(outputPath); trimmed != "" {
		return filepath.Clean(trimmed), nil
	}
	homePaths, err := deps.resolveHome()
	if err != nil {
		return "", fmt.Errorf("cli: resolve AGH home for Slack manifest: %w", err)
	}
	if err := deps.ensureHome(homePaths); err != nil {
		return "", fmt.Errorf("cli: ensure AGH home for Slack manifest: %w", err)
	}
	return filepath.Join(homePaths.HomeDir, bridgeManifestDefaultFile), nil
}

func encodeSlackManifest(manifest SlackManifestRecord) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(manifest); err != nil {
		return nil, fmt.Errorf("cli: encode Slack manifest: %w", err)
	}
	return output.Bytes(), nil
}
