package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newAccountEmailCommand(deps commandDeps) *cobra.Command {
	var news string
	cmd := accountVerb(
		deps,
		"email",
		"Set your product email preferences",
		func(cmd *cobra.Command, client accountCommandClient) error {
			result, err := client.UpdateAccountConsent(cmd.Context(), news == "on")
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
				if result.MarketingOptIn {
					return "Product news and offers: on", nil
				}
				return "Product news and offers: off", nil
			}})
		},
	)
	cmd.Flags().StringVar(&news, "news", "", "Set product news and offers (on or off)")
	cmd.PreRunE = func(_ *cobra.Command, _ []string) error {
		if news != "on" && news != cliOffValue {
			return newCLIUsageError("--news must be on or off")
		}
		return nil
	}
	return cmd
}

func newAccountTokenCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "token", Short: "Manage account tokens for local CI use"}
	cmd.AddCommand(
		newAccountTokenCreateCommand(deps),
		accountVerb(deps, "list", "List account tokens", func(cmd *cobra.Command, client accountCommandClient) error {
			result, err := client.ListAccountTokens(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{jsonValue: result.Tokens, human: func() (string, error) {
				lines := make([]string, 0, len(result.Tokens))
				for _, token := range result.Tokens {
					used := "never used"
					if token.LastUsedAt != nil {
						used = "last used " + accountTokenAge(deps.now().Sub(*token.LastUsedAt))
					}
					lines = append(lines, fmt.Sprintf("%s  %s  %s", token.ID, token.Name, used))
				}
				return strings.Join(lines, "\n"), nil
			}})
		}),
		newAccountTokenRevokeCommand(deps),
	)
	return cmd
}

func newAccountTokenCreateCommand(deps commandDeps) *cobra.Command {
	var name string
	cmd := accountVerb(
		deps,
		"create",
		"Create a local-use account token",
		func(cmd *cobra.Command, client accountCommandClient) error {
			result, err := client.CreateAccountToken(cmd.Context(), name)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
				return fmt.Sprintf("Account token %s (%s). Copy it now; it won't be shown again:\n%s\n"+
					"Use it with COMPOZY_ACCOUNT_TOKEN. It unlocks local use only.", result.ID, result.Name, result.Token), nil
			}})
		},
	)
	cmd.Flags().StringVar(&name, "name", "", "Name to show in the account token list")
	cmd.PreRunE = func(_ *cobra.Command, _ []string) error {
		if strings.TrimSpace(name) == "" {
			return newCLIUsageError("--name is required")
		}
		return nil
	}
	return cmd
}

func newAccountTokenRevokeCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{Use: "revoke <id>", Short: "Revoke an account token", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := accountClientFromDeps(deps)
			if err == nil {
				err = client.RevokeAccountToken(cmd.Context(), args[0])
			}
			if err != nil {
				return accountExecutionError{err}
			}
			return nil
		}}
}

func accountTokenAge(age time.Duration) string {
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%d min ago", age/time.Minute)
	case age < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", age/time.Hour)
	default:
		return fmt.Sprintf("%d days ago", age/(24*time.Hour))
	}
}

func newAccountDeleteCommand(deps commandDeps) *cobra.Command {
	var email string
	cmd := accountVerb(
		deps,
		"delete",
		"Delete your Compozy Cloud account",
		func(cmd *cobra.Command, client accountCommandClient) error {
			status, err := client.ReadAccountForDelete(cmd.Context())
			if err != nil {
				return err
			}
			writer := cmd.ErrOrStderr()
			if _, err = fmt.Fprintf(
				writer,
				"This deletes %s, unlinks %d computers and signs out every installation. Local data stays.\n",
				status.Email,
				status.RemoteComputersUsed,
			); err != nil {
				return err
			}
			if err := announceAccountDeletion(writer, status, deps.now()); err != nil {
				return err
			}
			confirmed := email
			if confirmed == "" {
				confirmed, err = readAccountDeletionConfirmation(cmd, writer)
				if err != nil {
					return err
				}
			}
			result, err := client.DeleteAccount(cmd.Context(), strings.TrimSpace(confirmed))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
				return accountDeletionResult(result), nil
			}})
		},
	)
	cmd.Flags().StringVar(&email, "confirm-email", "", "Confirm the exact account email without a terminal")
	cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
		if strings.TrimSpace(email) == "" && !deps.inputIsTerminal(cmd.InOrStdin()) {
			return newCLIUsageError("--confirm-email is required without a terminal")
		}
		return nil
	}
	return cmd
}

func announceAccountDeletion(writer io.Writer, status contract.AccountStatusResponse, now time.Time) error {
	sub := status.Subscription
	if sub == nil {
		return nil
	}
	lines := []string{"Deleting your account also cancels Pro."}
	if sub.Source == "complimentary" {
		_, err := fmt.Fprintln(writer, strings.Join(append(lines, "Your complimentary Pro ends now."), " "))
		return err
	}
	if sub.CurrentPeriodEnd != nil && sub.Interval != nil {
		interval := "monthly"
		if *sub.Interval == "year" {
			interval = "annual"
		}
		lines = append(lines, fmt.Sprintf("Pro (%s) is paid until %s and ends now.", interval,
			sub.CurrentPeriodEnd.Format(time.DateOnly)))
	}
	refund := "Refunds cover 14 days after a payment, so no refund applies."
	if eligible := sub.RefundEligibleUntil; eligible != nil && !now.After(*eligible) {
		refund = fmt.Sprintf("Your last payment was on %s, so it is refunded in full.",
			eligible.Add(-14*24*time.Hour).Format(time.DateOnly))
	}
	_, err := fmt.Fprintln(writer, strings.Join(lines, " ")+"\n"+refund)
	return err
}

func readAccountDeletionConfirmation(cmd *cobra.Command, writer io.Writer) (string, error) {
	if _, err := fmt.Fprint(writer, "Type your email to confirm: "); err != nil {
		return "", err
	}
	email, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if errors.Is(err, io.EOF) && email != "" {
		return email, nil
	}
	return email, err
}

func accountDeletionResult(result contract.AccountDeleteResponse) string {
	switch {
	case result.SubscriptionCancelled && result.Refund == "full":
		return "Account deleted. Pro was cancelled and your last payment will be refunded." //nolint:misspell // CLI contract.
	case result.SubscriptionCancelled:
		return "Account deleted. Pro was cancelled." //nolint:misspell // CLI contract.
	default:
		return "Account deleted."
	}
}
