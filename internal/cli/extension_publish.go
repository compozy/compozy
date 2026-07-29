package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	registrygithub "github.com/compozy/compozy/internal/registry/github"
	"github.com/spf13/cobra"
)

func newExtensionPublishCommand(deps commandDeps) *cobra.Command {
	var repository string
	var tag string
	var draft bool
	command := &cobra.Command{
		Use:   "publish [generation-directory]",
		Short: "Publish an immutable extension generation to GitHub Releases",
		Args:  optionalDirectoryArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			generationDir, err := extensionAuthoringDirectory(deps, args)
			if err != nil {
				return err
			}
			result, err := publishExtensionGeneration(cmd.Context(), deps, extensionpkg.PublishRequest{
				GenerationDir: generationDir,
				Repository:    repository,
				TagName:       tag,
				Draft:         draft,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionPublishBundle(result))
		},
	}
	command.Flags().StringVar(&repository, "repository", "", "GitHub repository in owner/name form")
	command.Flags().StringVar(&tag, "tag", "", "GitHub release tag")
	command.Flags().BoolVar(&draft, "draft", false, "Create or update a draft release")
	mustMarkFlagRequired(command, "repository")
	mustMarkFlagRequired(command, "tag")
	return command
}

func publishExtensionGeneration(
	ctx context.Context,
	deps commandDeps,
	request extensionpkg.PublishRequest,
) (_ *extensionpkg.PublishResult, err error) {
	token := strings.TrimSpace(deps.getenv("GITHUB_TOKEN"))
	if token == "" {
		return extensionpkg.PublishRelease(ctx, nil, request)
	}
	config, err := deps.loadConfig()
	if err != nil {
		return nil, fmt.Errorf("cli: load GitHub publish configuration: %w", err)
	}
	client := registrygithub.NewClient(
		config.Extensions.Sources.GitHub.BaseURL,
		registrygithub.WithToken(token),
	)
	defer func() {
		err = errors.Join(err, client.Close())
	}()
	return extensionpkg.PublishRelease(ctx, client, request)
}

func extensionPublishBundle(result *extensionpkg.PublishResult) outputBundle {
	return singleExtensionAuthoringBundle(
		result,
		"Extension release published",
		[]keyValue{
			{Label: "Release", Value: stringOrDash(result.ReleaseURL)},
			{Label: "Asset", Value: stringOrDash(result.AssetURL)},
			{Label: "SHA-256", Value: result.DigestSHA256},
		},
		"extension_publish",
		[]string{"release_url", "asset_url", "digest_sha256"},
		[]string{result.ReleaseURL, result.AssetURL, result.DigestSHA256},
	)
}
