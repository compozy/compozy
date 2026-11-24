package cli

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/sse"
	"github.com/compozy/compozy/internal/version"
	"github.com/spf13/cobra"
)

const (
	accountCommandName = "account"
	accountSignInHint  = "Sign in to use CompozyOS: compozy account login"
)

func newAccountCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: accountCommandName, Short: "Manage the Compozy Cloud account on this computer"}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return newCLIUsageError(err.Error()) })
	cmd.AddCommand(
		newAccountLoginCommand(deps),
		accountVerb(deps, "status", "Show account, plan, and device status", runAccountStatus),
		newAccountLogoutCommand(deps),
		newAccountUpgradeCommand(deps),
		newAccountEmailCommand(deps),
		newAccountTokenCommand(deps),
		newAccountDeleteCommand(deps),
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
			var usage interface{ cliExitCode() int }
			if errors.As(err, &usage) {
				return err
			}
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
			return accountSignInHint, nil
		}
		if status.TokenName != nil {
			return fmt.Sprintf("Signed in with account token %q", *status.TokenName), nil
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

func newAccountLogoutCommand(deps commandDeps) *cobra.Command {
	var unlink bool
	cmd := accountVerb(
		deps,
		"logout",
		"Sign out on this computer",
		func(cmd *cobra.Command, client accountCommandClient) error {
			result, err := client.LogoutAccount(cmd.Context(), unlink)
			if failure, ok := errors.AsType[*daemonAPIError](
				err,
			); ok &&
				failure.payload.Code == "logout_requires_unlink" {
				status, readErr := client.ReadAccount(cmd.Context())
				if readErr == nil && status.Email != "" {
					failure.payload.Error = fmt.Sprintf(
						"This computer is also linked to %s for remote access. Signing out unlinks it too.\n"+
							"Run again with --unlink to continue.",
						status.Email,
					)
				}
			}
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
				return "Signed out. CompozyOS needs you to sign in again before it can be used on this computer.", nil
			}})
		},
	)
	cmd.Flags().BoolVar(&unlink, "unlink", false, "Unlink this computer from remote access when signing out")
	return cmd
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

type accountLoginOptions struct {
	provider  string
	noBrowser bool
	news      *bool
}

func newAccountLoginCommand(deps commandDeps) *cobra.Command {
	var options accountLoginOptions
	var news, noNews bool
	cmd := accountVerb(
		deps,
		"login",
		"Sign in through a browser",
		func(cmd *cobra.Command, client accountCommandClient) error {
			input := options
			switch {
			case cmd.Flags().Changed("news"):
				input.news = &news
			case cmd.Flags().Changed("no-news"):
				input.news = new(!noNews)
			}
			return runAccountLogin(cmd, deps, client, input)
		},
	)
	cmd.Flags().StringVar(&options.provider, "provider", "", "Choose Google or GitHub")
	cmd.Flags().BoolVar(&options.noBrowser, "no-browser", false, "Print the activation URL without opening a browser")
	cmd.Flags().BoolVar(&news, "news", false, "Subscribe to product news and offers")
	cmd.Flags().BoolVar(&noNews, "no-news", false, "Turn off product news and offers")
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		if options.provider != "" && options.provider != "google" && options.provider != "github" {
			return newCLIUsageError("--provider must be google or github")
		}
		if cmd.Flags().Changed("news") && cmd.Flags().Changed("no-news") {
			return newCLIUsageError("use either --news or --no-news")
		}
		return nil
	}
	return cmd
}

func runAccountLogin(
	cmd *cobra.Command, deps commandDeps, client accountCommandClient, options accountLoginOptions,
) (returnErr error) {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	stream, err := openAccountLoginStream(ctx, deps, client)
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, stream.Close()) }()
	var input contract.AccountLoginRequest
	if options.provider != "" {
		input.Provider = new(options.provider)
	}
	attempt, err := client.StartAccountLogin(ctx, input)
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
	if err := announceAccountLogin(cmd, deps, attempt, options.noBrowser); err != nil {
		return err
	}
	newAccount, err := awaitAccountLogin(ctx, stream, attempt.AttemptID)
	if err != nil {
		return err
	}
	complete = true
	status, err := client.ReadAccount(ctx)
	if err != nil {
		return err
	}
	if !status.SignedIn {
		return cliSignInRequired()
	}
	if err := applyLoginNews(cmd, deps, client, options.news, newAccount); err != nil {
		return err
	}
	return writeCommandOutput(cmd, outputBundle{jsonValue: status.CLIStatus(), human: func() (string, error) {
		return fmt.Sprintf("Signed in as %s (%s)", status.Email, accountPlanLabel(status.Plan)), nil
	}})
}

func openAccountLoginStream(ctx context.Context, deps commandDeps, client accountCommandClient) (io.ReadCloser, error) {
	stream, err := client.OpenAccountSignIn(ctx)
	if err == nil || version.SignInRequired != "true" || ctx.Err() != nil {
		return stream, err
	}
	if _, ok := errors.AsType[*url.Error](err); !ok {
		return nil, err
	}
	if _, startErr := runDaemonDetached(ctx, deps); startErr != nil {
		return nil, errors.Join(err, startErr)
	}
	return client.OpenAccountSignIn(ctx)
}

func applyLoginNews(
	cmd *cobra.Command, deps commandDeps, client accountCommandClient, news *bool, newAccount bool,
) error {
	if news == nil && newAccount && deps.inputIsTerminal(cmd.InOrStdin()) {
		if _, err := fmt.Fprint(cmd.ErrOrStderr(), "Email me product news and offers? [y/N] "); err != nil {
			return err
		}
		answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		news = new(answer == "y" || answer == "yes")
	}
	if news == nil {
		return nil
	}
	_, err := client.UpdateAccountConsent(cmd.Context(), *news)
	return err
}

func announceAccountLogin(
	cmd *cobra.Command,
	deps commandDeps,
	attempt contract.AccountLoginResponse,
	noBrowser bool,
) error {
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
	if noBrowser {
		return nil
	}
	if err := deps.openBrowser(
		cmd.Context(),
		attempt.VerificationURIComplete,
	); err != nil &&
		cmd.Context().Err() != nil {
		return cmd.Context().Err()
	}
	return nil
}

func awaitAccountLogin(ctx context.Context, stream io.Reader, attemptID string) (bool, error) {
	var complete, newAccount bool
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
			newAccount = progress.NewAccount
			return sse.ErrStop
		case "expired", string(bootstrapPhaseFailed), "cancelled": //nolint:misspell // Public protocol state.
			return accountLoginFailure(progress.Code)
		default:
			return nil
		}
	})
	if err == nil && !complete {
		return false, &daemonAPIError{
			statusCode: http.StatusServiceUnavailable,
			payload: contract.ErrorPayload{
				Code: "account_operation_failed", Error: "Account sign-in updates ended. Try again.",
			},
		}
	}
	return newAccount, err
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
	case "installation_linked_to_other_account":
		return &daemonAPIError{statusCode: http.StatusConflict, payload: contract.ErrorPayload{
			Code:  code,
			Error: "This computer is linked for remote access. Unlink it before signing in with another account.",
		}}
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
