package cli

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/computers/sshsetup"
)

func newComputerAddSSHCommand(deps commandDeps) *cobra.Command {
	input := contract.AddComputerRequest{Kind: contract.ComputerAddKindSSH}
	cmd := computerSetupVerb(deps, "ssh <host>", "Set up a computer over SSH",
		func(cmd *cobra.Command, client computerSetupClient, arg string) error {
			input.Host = arg
			if cmd.Flags().Changed("port") && input.Port < 1 {
				return newCLIUsageError("SSH port must be between 1 and 65535.")
			}
			result, err := client.StartSSHComputer(cmd.Context(), input)
			if err != nil {
				return err
			}
			if mode, err := resolveOutputFormat(cmd); err != nil {
				return err
			} else if mode == OutputJSON || mode == OutputJSONL {
				return writeCommandOutput(cmd, outputBundle{jsonValue: result})
			}
			return waitComputerSetup(cmd, client, result.SetupID)
		})
	cmd.Flags().StringVar(&input.User, "user", "", "Override the SSH user")
	cmd.Flags().IntVar(&input.Port, "port", 0, "Override the SSH port")
	cmd.Flags().StringVar(&input.Name, "name", "", "Choose the name shown in this controller")
	cmd.Flags().
		StringVar(&input.TrustHostFingerprint, "trust-host-fingerprint", "", "Trust this exact SSH host fingerprint")
	return cmd
}

func newComputerUpdateCommand(deps commandDeps) *cobra.Command {
	return computerSetupVerb(deps, "update <name|id>", "Update a computer over SSH",
		func(cmd *cobra.Command, client computerSetupClient, arg string) error {
			item, err := resolveComputer(cmd.Context(), client, arg)
			if err != nil {
				return err
			}
			result, err := client.StartComputerUpdate(cmd.Context(), item.ID)
			if err != nil {
				return err
			}
			if mode, err := resolveOutputFormat(cmd); err != nil {
				return err
			} else if mode == OutputJSON || mode == OutputJSONL {
				return writeCommandOutput(cmd, outputBundle{jsonValue: result})
			}
			return waitComputerSetup(cmd, client, result.SetupID)
		})
}

func newComputerSetupCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "setup", Short: "Manage an SSH computer setup"}
	cmd.AddCommand(
		computerSetupVerb(deps, "answer <setup_id>", "Answer the current SSH prompt from a terminal",
			answerComputerSetup),
		computerSetupVerb(deps, "status <setup_id>", "Read the current SSH setup status",
			func(cmd *cobra.Command, client computerSetupClient, id string) error {
				result, err := client.GetComputerSetup(cmd.Context(), id, 0)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
					return computerSetupStatusLine(result), nil
				}})
			}),
		computerSetupVerb(deps, "cancel <setup_id>", "Stop the setup and release its slot",
			func(cmd *cobra.Command, client computerSetupClient, id string) error {
				result, err := client.CancelComputerSetup(cmd.Context(), id)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, outputBundle{jsonValue: result, human: func() (string, error) {
					return "Setup cancelled.", nil //nolint:misspell // Public wording.
				}})
			}),
	)
	return cmd
}

func computerSetupVerb(
	deps commandDeps, use, short string, run func(*cobra.Command, computerSetupClient, string) error,
) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := computerClientFromDeps(deps)
			if err != nil {
				return err
			}
			setups, ok := client.(computerSetupClient)
			if !ok {
				return errors.New("daemon client does not support SSH setups")
			}
			if err := run(cmd, setups, args[0]); err != nil {
				return accountExecutionError{err}
			}
			return nil
		}}
}

func waitComputerSetup(cmd *cobra.Command, client computerSetupClient, id string) error {
	status, err := client.GetComputerSetup(cmd.Context(), id, 0)
	previous := ""
	for err == nil {
		switch status.State {
		case sshsetup.StepDone:
			if status.ComputerID == nil {
				return errors.New("setup finished without a computer ID")
			}
			item, err := client.GetComputer(cmd.Context(), *status.ComputerID)
			if err != nil {
				return err
			}
			return writeComputerStatus(cmd, item)
		case sshsetup.Failed:
			if status.Error == nil {
				return errors.New("SSH setup failed")
			}
			return &daemonAPIError{statusCode: http.StatusUnprocessableEntity, payload: *status.Error}
		case sshsetup.Canceled:
			return writeCommandOutput(cmd, outputBundle{jsonValue: status, human: func() (string, error) {
				return computerSetupStatusLine(status), nil
			}})
		case sshsetup.NeedsInput, sshsetup.NeedsOperatorSecret:
			if err := answerSetupPrompt(cmd, client, status); err != nil {
				return err
			}
		}
		if status.Step != previous {
			if _, err := fmt.Fprintln(cmd.ErrOrStderr(), computerSetupStep(status.Step)); err != nil {
				return err
			}
			previous = status.Step
		}
		status, err = client.GetComputerSetup(cmd.Context(), id, sshsetup.MaxStatusWait)
	}
	return err
}

func computerSetupStatusLine(status contract.ComputerSetupStatus) string {
	line := fmt.Sprintf("%s  %s  %s", status.SetupID, status.Step, status.State)
	if status.Error != nil {
		line += "\n" + status.Error.Error
	}
	return line
}

func computerSetupStep(step string) string {
	switch step {
	case sshsetup.StepAdmit:
		return "Checking your plan"
	case sshsetup.StepVerify:
		return "Verifying release"
	case sshsetup.StepConnect, sshsetup.StepForward:
		return "Connecting"
	case sshsetup.StepInstall:
		return "Installing Compozy"
	case sshsetup.StepService:
		return "Starting Compozy service"
	case sshsetup.StepLink:
		return "Linking to your account"
	default:
		return step
	}
}

func answerComputerSetup(cmd *cobra.Command, client computerSetupClient, id string) error {
	status, err := client.GetComputerSetup(cmd.Context(), id, 0)
	if err != nil {
		return err
	}
	if err := answerSetupPrompt(cmd, client, status); err != nil {
		return err
	}
	return writeCommandOutput(cmd, outputBundle{
		jsonValue: map[string]bool{"answered": true},
		human:     func() (string, error) { return "Answered SSH setup prompt.", nil },
	})
}

func answerSetupPrompt(cmd *cobra.Command, client computerSetupClient, status contract.ComputerSetupStatus) error {
	if status.Prompt == nil {
		return errors.New("this setup is not waiting for an answer")
	}
	tty, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(tty.Fd())) {
		return fmt.Errorf("%s: answer the SSH prompt from the app or a terminal", status.SetupID)
	}
	if status.Prompt.Kind == sshsetup.TrustHost {
		if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Host key %s for %s — trust it? [y/N] ",
			status.Prompt.Fingerprint, status.Prompt.Host); err != nil {
			return err
		}
		text, err := bufio.NewReader(tty).ReadString('\n')
		if err != nil {
			return err
		}
		accept := strings.EqualFold(strings.TrimSpace(text), "y")
		return client.AnswerComputerSetup(cmd.Context(), status.SetupID, contract.ComputerSetupAnswer{
			Kind: sshsetup.TrustHost, Accept: &accept,
		})
	}
	if _, err := fmt.Fprint(cmd.ErrOrStderr(), "SSH password or code: "); err != nil {
		return err
	}
	secret, err := term.ReadPassword(int(tty.Fd()))
	if err != nil {
		return err
	}
	defer clear(secret)
	if _, err := fmt.Fprintln(cmd.ErrOrStderr()); err != nil {
		return err
	}
	return client.AnswerComputerSetup(cmd.Context(), status.SetupID, contract.ComputerSetupAnswer{
		Kind: sshsetup.Secret, Value: string(secret),
	})
}
