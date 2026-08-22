package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func previewExtensionInstallPlan(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	plan extensionInstallPlan,
) (*ExtensionInstallPreviewRecord, error) {
	mode, err := resolveInheritedOutputFormat(cmd)
	if err != nil {
		return nil, err
	}
	if mode != OutputHuman {
		return nil, nil
	}
	client, running, err := daemonClientIfRunning(ctx, deps)
	if err != nil {
		return nil, err
	}
	if !running {
		return nil, nil
	}
	for index, request := range plan.Attempts {
		preview, previewErr := client.PreviewExtensionInstall(ctx, request)
		if previewErr == nil {
			return &preview, nil
		}
		if index == len(plan.Attempts)-1 || !extensionInstallFallbackAllowed(previewErr) {
			return nil, previewErr
		}
	}
	return nil, errors.New("cli: extension install preview plan exhausted")
}

func writeExtensionInstallPreview(cmd *cobra.Command, preview *ExtensionInstallPreviewRecord) error {
	if preview == nil {
		return nil
	}
	var output strings.Builder
	name := strings.TrimSpace(preview.Name)
	if name == "" {
		name = "Extension"
	}
	fmt.Fprintf(&output, "%s will:\n", name)
	for _, profile := range preview.DeclaredProfiles {
		action := "bind profile "
		if profile.Create {
			action = "create profile "
		}
		fmt.Fprintf(&output, "  • %s%s", action, profile.Name)
		if len(profile.Credentials) > 0 {
			credentials := make([]string, 0, len(profile.Credentials))
			for _, requirement := range profile.Credentials {
				credentials = append(credentials, requirement.Provider+" "+requirement.Slot)
			}
			fmt.Fprintf(&output, " (needs %s)", strings.Join(credentials, ", "))
		}
		output.WriteByte('\n')
	}
	for _, placement := range preview.Placements {
		target := "every profile"
		if strings.TrimSpace(placement.Profile) != "" {
			target = "profile " + placement.Profile
		}
		fmt.Fprintf(&output, "  • add %s %s to %s\n", placement.Kind, placement.Resource, target)
	}
	if _, err := fmt.Fprint(cmd.ErrOrStderr(), output.String()); err != nil {
		return fmt.Errorf("cli: write extension install preview: %w", err)
	}
	return nil
}
