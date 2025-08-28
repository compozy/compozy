package handlers

import (
	"context"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/compozy/compozy/cli/cmd"
	bootstrapcli "github.com/compozy/compozy/cli/cmd/auth/bootstrap"
	"github.com/compozy/compozy/cli/tui/styles"
	"github.com/compozy/compozy/engine/auth/bootstrap"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/spf13/cobra"
)

// BootstrapTUI handles the bootstrap command in TUI mode
func BootstrapTUI(ctx context.Context, cobraCmd *cobra.Command, _ *cmd.CommandExecutor, _ []string) error {
	// Parse flags
	flags, err := parseTUIBootstrapFlags(cobraCmd)
	if err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	// Handle check status
	if flags.check {
		return handleTUIStatusCheck(ctx)
	}

	// Check existing bootstrap and get user decisions
	shouldContinue, updatedFlags, err := checkAndPromptBootstrap(ctx, flags)
	if err != nil {
		return err
	}
	if !shouldContinue {
		return nil
	}

	// Execute bootstrap
	return executeTUIBootstrap(ctx, updatedFlags)
}

// parseTUIBootstrapFlags extracts flags from cobra command
func parseTUIBootstrapFlags(cmd *cobra.Command) (*bootstrapFlags, error) {
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

// handleTUIStatusCheck displays bootstrap status in TUI mode
func handleTUIStatusCheck(ctx context.Context) error {
	factory := &bootstrapcli.DefaultServiceFactory{}
	service, cleanup, err := factory.CreateService(ctx)
	if err != nil {
		fmt.Println(styles.ErrorStyle.Render("❌ Failed to create service: " + err.Error()))
		return err
	}
	defer cleanup()

	status, err := service.CheckBootstrapStatus(ctx)
	if err != nil {
		fmt.Println(styles.ErrorStyle.Render("❌ Failed to check status: " + err.Error()))
		return err
	}

	if status.IsBootstrapped {
		fmt.Println(styles.SuccessStyle.Render("✅ System is bootstrapped"))
		fmt.Printf("   Admin users: %d\n", status.AdminCount)
		fmt.Printf("   Total users: %d\n", status.UserCount)
	} else {
		fmt.Println(styles.WarningStyle.Render("⚠️  System is not bootstrapped"))
		fmt.Println("   Run 'compozy auth bootstrap' to create the initial admin user")
	}
	return nil
}

// checkAndPromptBootstrap checks existing bootstrap and prompts user
func checkAndPromptBootstrap(ctx context.Context, flags *bootstrapFlags) (bool, *bootstrapFlags, error) {
	factory := &bootstrapcli.DefaultServiceFactory{}
	service, cleanup, err := factory.CreateService(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("failed to create service: %w", err)
	}
	defer cleanup()

	// Check current status
	status, err := service.CheckBootstrapStatus(ctx)
	if err != nil {
		// Log the error and inform user
		logger.FromContext(ctx).Warn("Failed to check bootstrap status", "error", err)
		fmt.Println(styles.WarningStyle.Render("⚠️  Could not verify bootstrap status"))
		fmt.Println(styles.HelpStyle.Render("   This might be due to database connectivity issues"))

		// Ask user if they want to continue
		var continueAnyway bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Do you want to continue anyway?").
					Description("This might create duplicate admin users if the system is already bootstrapped").
					Value(&continueAnyway),
			),
		)
		if err := form.Run(); err != nil || !continueAnyway {
			return false, nil, fmt.Errorf("bootstrap canceled due to status check failure")
		}
		// User chose to continue despite the error
		status = &bootstrap.Status{IsBootstrapped: false}
	} else if status.IsBootstrapped && !flags.force {
		if !promptForAdditionalAdmin(status) {
			fmt.Println(styles.HelpStyle.Render("Bootstrap canceled"))
			return false, nil, nil
		}
		flags.force = true
	}

	// Get email if not provided
	if flags.email == "" {
		email, err := promptForEmail()
		if err != nil {
			return false, nil, err
		}
		flags.email = email
	}

	// Validate email
	if err := bootstrapcli.ValidateEmail(flags.email); err != nil {
		return false, nil, fmt.Errorf("invalid email: %w", err)
	}

	// Get final confirmation
	if !flags.force && !confirmBootstrap(flags.email) {
		fmt.Println(styles.HelpStyle.Render("Bootstrap canceled"))
		return false, nil, nil
	}

	return true, flags, nil
}

// promptForAdditionalAdmin prompts user about creating additional admin
func promptForAdditionalAdmin(status *bootstrap.Status) bool {
	fmt.Println(styles.WarningStyle.Render("⚠️  System is already bootstrapped"))
	fmt.Printf("   Admin users: %d\n", status.AdminCount)
	fmt.Printf("   Total users: %d\n", status.UserCount)

	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Do you want to create another admin user?").
				Value(&confirm),
		),
	)

	if err := form.Run(); err != nil {
		return false
	}
	return confirm
}

// promptForEmail prompts the user for an email address
func promptForEmail() (string, error) {
	var email string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Admin Email Address").
				Description("Enter the email for the admin user").
				Placeholder("admin@company.com").
				Value(&email).
				Validate(bootstrapcli.ValidateEmail),
		),
	)

	if err := form.Run(); err != nil {
		return "", err
	}
	return email, nil
}

// confirmBootstrap asks for final confirmation
func confirmBootstrap(email string) bool {
	fmt.Println(styles.TitleStyle.Render("Bootstrap Configuration"))
	fmt.Printf("   Email: %s\n", email)
	fmt.Printf("   Role:  admin\n\n")

	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Create initial admin user?").
				Value(&confirm).
				Affirmative("Yes").
				Negative("No"),
		),
	)

	if err := form.Run(); err != nil {
		return false
	}
	return confirm
}

// executeTUIBootstrap performs the bootstrap operation
func executeTUIBootstrap(ctx context.Context, flags *bootstrapFlags) error {
	fmt.Println(styles.InfoStyle.Render("Creating admin user..."))

	factory := &bootstrapcli.DefaultServiceFactory{}
	service, cleanup, err := factory.CreateService(ctx)
	if err != nil {
		fmt.Println(styles.ErrorStyle.Render("❌ Failed to create service: " + err.Error()))
		return err
	}
	defer cleanup()

	result, err := service.BootstrapAdmin(ctx, &bootstrap.Input{
		Email: flags.email,
		Force: flags.force,
	})

	if err != nil {
		if coreErr, ok := err.(*core.Error); ok {
			fmt.Println(styles.ErrorStyle.Render(fmt.Sprintf("❌ %s: %s", coreErr.Code, coreErr.Message)))
		} else {
			fmt.Println(styles.ErrorStyle.Render("❌ Bootstrap failed: " + err.Error()))
		}
		return err
	}

	displaySuccess(result)
	return nil
}

// displaySuccess shows the success message and API key
func displaySuccess(result *bootstrap.Result) {
	fmt.Println()
	fmt.Println(styles.SuccessStyle.Render("✅ Admin user created successfully!"))
	fmt.Println()
	fmt.Println(styles.TitleStyle.Render("User Details:"))
	fmt.Printf("   ID:    %s\n", result.UserID)
	fmt.Printf("   Email: %s\n", result.Email)
	fmt.Printf("   Role:  admin\n")
	fmt.Println()
	fmt.Println(styles.TitleStyle.Render("API Key:"))
	fmt.Println(styles.BadgeStyle.Render(result.APIKey))
	fmt.Println()
	fmt.Println(styles.WarningStyle.Render("⚠️  IMPORTANT: Save this API key securely!"))
	fmt.Println(styles.WarningStyle.Render("   It will not be shown again."))
	fmt.Println()
	fmt.Println(styles.HelpStyle.Render("You can now use this API key to authenticate with:"))
	fmt.Println(styles.CodeStyle.Render("export COMPOZY_API_KEY=" + result.APIKey))
}
