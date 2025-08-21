package handlers

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/cli/cmd"
	bootstrapcli "github.com/compozy/compozy/cli/cmd/auth/bootstrap"
	"github.com/compozy/compozy/engine/auth/bootstrap"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// BootstrapJSON handles the bootstrap command in JSON mode
func BootstrapJSON(ctx context.Context, cobraCmd *cobra.Command, _ *cmd.CommandExecutor, _ []string) error {
	// Parse flags
	flags, err := parseBootstrapFlags(cobraCmd)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to parse flags: %v", err))
	}

	// Handle check status
	if flags.check {
		return handleJSONStatusCheck(ctx)
	}

	// Validate and execute bootstrap
	if flags.email == "" {
		return outputJSONError("email is required in JSON mode")
	}

	if err := bootstrapcli.ValidateEmail(flags.email); err != nil {
		return outputJSONError(fmt.Sprintf("invalid email: %v", err))
	}

	result, err := executeBootstrap(ctx, flags.email, flags.force)
	if err != nil {
		if coreErr, ok := err.(*core.Error); ok {
			return outputJSONError(fmt.Sprintf("%s: %s", coreErr.Code, coreErr.Message))
		}
		return outputJSONError(fmt.Sprintf("Bootstrap failed: %v", err))
	}

	return outputBootstrapResult(result)
}

// bootstrapFlags holds the parsed command flags
type bootstrapFlags struct {
	email string
	force bool
	check bool
}

// parseBootstrapFlags extracts flags from the cobra command
func parseBootstrapFlags(cmd *cobra.Command) (*bootstrapFlags, error) {
	email, err := cmd.Flags().GetString("email")
	if err != nil {
		return nil, fmt.Errorf("failed to get email flag: %w", err)
	}

	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return nil, fmt.Errorf("failed to get force flag: %w", err)
	}

	check, err := cmd.Flags().GetBool("check")
	if err != nil {
		return nil, fmt.Errorf("failed to get check flag: %w", err)
	}

	return &bootstrapFlags{
		email: email,
		force: force,
		check: check,
	}, nil
}

// handleJSONStatusCheck handles the bootstrap status check in JSON mode
func handleJSONStatusCheck(ctx context.Context) error {
	factory := &bootstrapcli.DefaultServiceFactory{}
	service, cleanup, err := factory.CreateService(ctx)
	if err != nil {
		return outputJSONError(fmt.Sprintf("failed to create service: %v", err))
	}
	defer cleanup()

	status, err := service.CheckBootstrapStatus(ctx)
	if err != nil {
		return outputJSONError(err.Error())
	}

	return outputJSONResponse(map[string]any{
		"bootstrapped": status.IsBootstrapped,
		"admin_count":  status.AdminCount,
		"user_count":   status.UserCount,
	})
}

// executeBootstrap performs the bootstrap operation
func executeBootstrap(ctx context.Context, email string, force bool) (*bootstrap.Result, error) {
	factory := &bootstrapcli.DefaultServiceFactory{}
	service, cleanup, err := factory.CreateService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}
	defer cleanup()

	log := logger.FromContext(ctx)
	log.Info("Bootstrapping initial admin user", "email", email)

	return service.BootstrapAdmin(ctx, &bootstrap.Input{
		Email: email,
		Force: force,
	})
}

// outputBootstrapResult formats and outputs the bootstrap result
func outputBootstrapResult(result *bootstrap.Result) error {
	return outputJSONResponse(map[string]any{
		"success": true,
		"message": "Admin user created successfully",
		"user": map[string]any{
			"id":    result.UserID,
			"email": result.Email,
			"role":  "admin",
		},
		"api_key": result.APIKey,
		"warning": "SAVE THIS API KEY - it will not be shown again!",
	})
}
