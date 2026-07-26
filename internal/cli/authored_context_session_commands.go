package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"strings"

	"github.com/spf13/cobra"
)

func newSessionSoulCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   authoredContextSoulKey,
		Short: "Manage session Soul snapshots",
	}
	cmd.AddCommand(newSessionSoulRefreshCommand(deps))
	return cmd
}

func newSessionSoulRefreshCommand(deps commandDeps) *cobra.Command {
	var (
		expectedDigest string
		idempotencyKey string
	)
	cmd := &cobra.Command{
		Use:     "refresh <session-id>",
		Short:   "Refresh an idle session's Soul snapshot",
		Example: "  agh session soul refresh sess_123 --expected-digest sha256:old --json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			digest, err := changedStringFlag(cmd, "expected-digest", expectedDigest)
			if err != nil {
				return err
			}
			record, err := client.RefreshSessionSoul(cmd.Context(), args[0], SessionSoulRefreshRequest{
				ExpectedDigest: digest,
				IdempotencyKey: strings.TrimSpace(idempotencyKey),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentSoulBundle(record))
		},
	}
	cmd.Flags().StringVar(&expectedDigest, "expected-digest", "", "Expected current session Soul digest for CAS")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	return cmd
}

func newSessionHealthCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "health <session-id>",
		Short:   "Read session health and wake eligibility",
		Example: "  agh session health sess_123 --json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.GetSessionHealth(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionHealthBundle(record))
		},
	}
	return cmd
}

func newSessionInspectCommand(deps commandDeps) *cobra.Command {
	var includeEvents bool
	cmd := &cobra.Command{
		Use:     "inspect <session-id>",
		Short:   "Inspect session health, wake audit, and policy correlation",
		Example: "  agh session inspect sess_123 --include-wake-events --json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.InspectSession(cmd.Context(), args[0], SessionInspectQuery{
				IncludeRecentWakeEvents: includeEvents,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionInspectBundle(record))
		},
	}
	cmd.Flags().BoolVar(&includeEvents, "include-wake-events", false, "Include recent wake audit rows")
	return cmd
}

func addWorkspaceFlag(cmd *cobra.Command) {
	cmd.Flags().String(workspaceSkillSource, "", "Resolve the agent from a workspace id, name, or path")
}

func addAuthoredBodyFlags(cmd *cobra.Command, input *authoredBodyInput) {
	cmd.Flags().StringVar(&input.file, "file", "", "Read authored context body from a file")
	cmd.Flags().BoolVar(&input.stdin, "stdin", false, "Read authored context body from stdin")
}

func readAuthoredBody(cmd *cobra.Command, input authoredBodyInput, required bool) (string, error) {
	if cmd.Flags().Changed("file") && input.stdin {
		return "", errors.New("cli: --file and --stdin cannot be combined")
	}
	if input.stdin {
		body, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", fmt.Errorf("cli: read authored body from stdin: %w", err)
		}
		return string(body), nil
	}
	if cmd.Flags().Changed("file") {
		path := strings.TrimSpace(input.file)
		if path == "" {
			return "", errors.New("cli: --file cannot be empty")
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("cli: read authored body file: %w", err)
		}
		return string(body), nil
	}
	if required {
		return "", errors.New("cli: provide authored body with --file or --stdin")
	}
	return "", nil
}

func changedStringFlag(cmd *cobra.Command, name string, value string) (string, error) {
	if !cmd.Flags().Changed(name) {
		return "", fmt.Errorf("cli: --%s is required", name)
	}
	return strings.TrimSpace(value), nil
}

func optionalStringFlag(cmd *cobra.Command, name string, value string) string {
	if !cmd.Flags().Changed(name) {
		return ""
	}
	return strings.TrimSpace(value)
}

func optionalExpectedDigestFlag(cmd *cobra.Command, expectedDigest string, ifMatchDigest string) (string, error) {
	expectedChanged := cmd.Flags().Changed("expected-digest")
	ifMatchChanged := cmd.Flags().Changed("if-match")
	if expectedChanged && ifMatchChanged {
		return "", errors.New("cli: use only one of --expected-digest or --if-match")
	}
	if ifMatchChanged {
		return strings.TrimSpace(ifMatchDigest), nil
	}
	if expectedChanged {
		return strings.TrimSpace(expectedDigest), nil
	}
	return "", nil
}

func changedExpectedDigestFlag(cmd *cobra.Command, expectedDigest string, ifMatchDigest string) (string, error) {
	digest, err := optionalExpectedDigestFlag(cmd, expectedDigest, ifMatchDigest)
	if err != nil {
		return "", err
	}
	if !cmd.Flags().Changed("expected-digest") && !cmd.Flags().Changed("if-match") {
		return "", errors.New("cli: --expected-digest or --if-match is required")
	}
	return digest, nil
}

func changedNonEmptyStringFlag(cmd *cobra.Command, name string, value string) (string, error) {
	trimmed, err := changedStringFlag(cmd, name, value)
	if err != nil {
		return "", err
	}
	if trimmed == "" {
		return "", fmt.Errorf("cli: --%s cannot be empty", name)
	}
	return trimmed, nil
}
