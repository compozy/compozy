package cli

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/sse"
	"github.com/spf13/cobra"
)

func newAccountCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "account", Short: "Manage the Compozy account on this computer"}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return newCLIUsageError(err.Error()) })
	cmd.AddCommand(
		accountVerb(
			deps,
			"login",
			"Sign in through a browser",
			func(cmd *cobra.Command, client accountCommandClient) error {
				return runAccountLogin(cmd, deps, client)
			},
		),
		accountVerb(deps, "status", "Show account, plan, and device status", runAccountStatus),
		accountVerb(deps, "logout", "Sign out on this computer", runAccountLogout),
		newAccountUpgradeCommand(deps),
	)
	return cmd
}

func accountVerb(
	deps commandDeps,
	verb, short string,
	run func(*cobra.Command, accountCommandClient) error,
) *cobra.Command {
	return &cobra.Command{Use: verb, Short: short, Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.NoArgs(cmd, args); err != nil {
			return newCLIUsageError(err.Error())
		}
		return nil
	}, RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := accountClientFromDeps(deps)
		if err == nil {
			err = run(cmd, client)
		}
		if err != nil {
			return accountExecutionError{err}
		}
		return nil
	}}
}

type accountExecutionError struct{ error }

func (e accountExecutionError) Unwrap() error  { return e.error }
func (accountExecutionError) cliExitCode() int { return 1 }

func runAccountStatus(cmd *cobra.Command, client accountCommandClient) error {
	status, err := client.ReadAccount(cmd.Context())
	if err != nil {
		return err
	}
	result := status.CLIStatus()
	return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
		if !status.SignedIn {
			return "Sign in to use Compozy: compozy account login", nil
		}
		plan := accountPlanLabel(result.Plan)
		if result.RemoteComputersLimit != nil {
			plan += fmt.Sprintf(
				" (%d of %d remote computers used)",
				result.RemoteComputersUsed,
				*result.RemoteComputersLimit,
			)
		} else if result.Plan != "" {
			plan += fmt.Sprintf(" (%d remote computers used)", result.RemoteComputersUsed)
		}
		text := fmt.Sprintf("Account   %s\nPlan      %s", result.Email, plan)
		if result.Device != nil {
			text += fmt.Sprintf("\nDevice    %s (%s)", result.Device.Name, result.Device.ID)
			if result.Device.Admin {
				text += " — trusted for admin"
			}
		}
		return text, nil
	}})
}

func runAccountLogout(cmd *cobra.Command, client accountCommandClient) error {
	result, err := client.LogoutAccount(cmd.Context())
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
		return "Signed out. Compozy needs you to sign in again before it can be used on this computer.", nil
	}})
}

func newAccountUpgradeCommand(deps commandDeps) *cobra.Command {
	var annual bool
	cmd := accountVerb(
		deps,
		"upgrade",
		"Open a Pro checkout in your browser",
		func(cmd *cobra.Command, client accountCommandClient) error {
			interval := "month"
			if annual {
				interval = "year"
			}
			result, err := client.CheckoutAccount(cmd.Context(), interval)
			if err != nil {
				return err
			}
			if !validAccountBrowserURL(result.URL) {
				return errors.New("invalid checkout URL")
			}
			message := fmt.Sprintf("Opening checkout in your browser… (%s)", result.URL)
			if err := deps.openBrowser(cmd.Context(), result.URL); err != nil {
				message = "Open " + result.URL
			}
			return writeCommandOutput(
				cmd,
				outputBundle{jsonValue: result, human: func() (string, error) { return message, nil }},
			)
		},
	)
	cmd.Flags().BoolVar(&annual, "annual", false, "Choose annual billing")
	return cmd
}

func runAccountLogin(cmd *cobra.Command, deps commandDeps, client accountCommandClient) (returnErr error) {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	stream, err := client.OpenAccountSignIn(ctx)
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, stream.Close()) }()
	attempt, err := client.StartAccountLogin(ctx)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		if complete {
			return
		}
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		returnErr = errors.Join(returnErr, client.CancelAccountLogin(cleanup, attempt.AttemptID))
	}()
	if err := announceAccountLogin(cmd, deps, attempt); err != nil {
		return err
	}
	if err := awaitAccountLogin(ctx, stream, attempt.AttemptID); err != nil {
		return err
	}
	complete = true
	status, err := client.ReadAccount(ctx)
	if err != nil {
		return err
	}
	if !status.SignedIn {
		return errors.New("Sign in to use Compozy: compozy account login")
	}
	return writeCommandOutput(cmd, outputBundle{jsonValue: status.CLIStatus(), human: func() (string, error) {
		return fmt.Sprintf("Signed in as %s (%s)", status.Email, accountPlanLabel(status.Plan)), nil
	}})
}

func announceAccountLogin(cmd *cobra.Command, deps commandDeps, attempt contract.AccountLoginResponse) error {
	if !validAccountBrowserURL(attempt.VerificationURIComplete) {
		return errors.New("invalid activation URL")
	}
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	writer := cmd.OutOrStdout()
	if mode != OutputHuman {
		writer = cmd.ErrOrStderr()
	}
	if _, err := fmt.Fprintf(
		writer,
		"Open %s\nCheck that the page shows code %s, then continue with Google or GitHub.\nWaiting for sign-in…\n",
		attempt.VerificationURIComplete,
		attempt.UserCode,
	); err != nil {
		return err
	}
	// The printed URL is also the continuation path on a headless computer.
	if err := deps.openBrowser(cmd.Context(), attempt.VerificationURIComplete); err != nil && cmd.Context().Err() != nil {
		return cmd.Context().Err()
	}
	return nil
}

func awaitAccountLogin(ctx context.Context, stream io.Reader, attemptID string) error {
	var complete bool
	err := sse.Decode(ctx, stream, func(event sse.Event) error {
		if event.Event != contract.SignInProgressEvent {
			return nil
		}
		var progress contract.AccountSignInProgress
		if err := json.Unmarshal(event.Data, &progress); err != nil {
			return err
		}
		if progress.AttemptID != attemptID {
			return nil
		}
		switch progress.State {
		case "signed_in":
			complete = true
			return sse.ErrStop
		case "expired", "failed", "cancelled": //nolint:misspell // Public protocol state.
			return accountLoginFailure(progress.Code)
		default:
			return nil
		}
	})
	if err == nil && !complete {
		return errors.New("Account sign-in updates ended. Try again.")
	}
	return err
}

func accountLoginFailure(code string) error {
	message := "Sign-in failed. Try again."
	switch code {
	case "account_login_expired":
		message = "Sign-in timed out. Try again."
	case "account_cloud_unreachable":
		message = "Can't reach Compozy Cloud. Local work is unaffected."
	case "account_suspended":
		message = "This account is suspended. Contact support@compozy.com."
	case "", "access_denied":
		code, message = "access_denied", "Sign-in was cancelled." //nolint:misspell // Spec copy.
	}
	return &daemonAPIError{
		statusCode: http.StatusUnauthorized,
		payload:    contract.ErrorPayload{Code: code, Error: message},
	}
}

func accountPlanLabel(plan string) string {
	switch plan {
	case "free":
		return "Free"
	case "pro":
		return "Pro"
	default:
		return "Unknown"
	}
}

func validAccountBrowserURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Hostname() != "" &&
		parsed.User == nil
}
