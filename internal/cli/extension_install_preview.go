package cli

import (
	"context"
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
	preview, err := runExtensionInstallAttempts(
		plan.Attempts,
		func(request InstallExtensionRequest) (ExtensionInstallPreviewRecord, error) {
			return client.PreviewExtensionInstall(ctx, request)
		},
	)
	if err != nil {
		return nil, err
	}
	return &preview, nil
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
	if _, err := fmt.Fprintf(&output, "%s will:\n", name); err != nil {
		return fmt.Errorf("cli: format extension install preview: %w", err)
	}
	for _, profile := range preview.DeclaredProfiles {
		action := "bind profile "
		if profile.Create {
			action = "create profile "
		}
		if _, err := fmt.Fprintf(&output, "  • %s%s", action, profile.Name); err != nil {
			return fmt.Errorf("cli: format extension install preview profile: %w", err)
		}
		if len(profile.Credentials) > 0 {
			credentials := make([]string, 0, len(profile.Credentials))
			for _, requirement := range profile.Credentials {
				credentials = append(credentials, requirement.Provider+" "+requirement.Slot)
			}
			if _, err := fmt.Fprintf(&output, " (needs %s)", strings.Join(credentials, ", ")); err != nil {
				return fmt.Errorf("cli: format extension install preview credentials: %w", err)
			}
		}
		if err := output.WriteByte('\n'); err != nil {
			return fmt.Errorf("cli: format extension install preview line: %w", err)
		}
	}
	for _, placement := range preview.Placements {
		target := "every profile"
		if strings.TrimSpace(placement.Profile) != "" {
			target = "profile " + placement.Profile
		}
		if _, err := fmt.Fprintf(
			&output,
			"  • add %s %s to %s\n",
			placement.Kind,
			placement.Resource,
			target,
		); err != nil {
			return fmt.Errorf("cli: format extension install preview placement: %w", err)
		}
	}
	if _, err := fmt.Fprint(cmd.ErrOrStderr(), output.String()); err != nil {
		return fmt.Errorf("cli: write extension install preview: %w", err)
	}
	return nil
}
