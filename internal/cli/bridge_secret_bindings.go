package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	bridgeSecretValueFileFlag  = "secret-value-file"
	bridgeSecretValueMaxBytes  = 4 << 20
	bridgeSecretValueStdinFlag = "secret-value-stdin"
)

func newBridgeSecretBindingsCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret-bindings",
		Short: "Manage bridge secret bindings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newBridgeSecretBindingsListCommand(deps))
	cmd.AddCommand(newBridgeSecretBindingsPutCommand(deps))
	cmd.AddCommand(newBridgeSecretBindingsDeleteCommand(deps))
	return cmd
}

func newBridgeSecretBindingsListCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "list <id>",
		Short: "List secret bindings for one bridge instance",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			items, err := client.ListBridgeSecretBindings(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeSecretBindingListBundle(items))
		},
	}
}

func newBridgeSecretBindingsPutCommand(deps commandDeps) *cobra.Command {
	var request BridgeSecretBindingRequest
	var secretValueFromStdin bool
	var secretValueFile string
	cmd := &cobra.Command{
		Use:   "put <id> <binding-name>",
		Short: "Create or update one bridge secret binding",
		Args:  exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			request.SecretRef = strings.TrimSpace(request.SecretRef)
			request.Kind = strings.TrimSpace(request.Kind)
			secretValue, err := bridgeSecretValueInput(cmd, secretValueFromStdin, secretValueFile)
			if err != nil {
				return err
			}
			request.SecretValue = secretValue
			if request.SecretRef == "" {
				return errors.New("cli: --secret-ref is required")
			}
			if request.Kind == "" {
				return errors.New("cli: --kind is required")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			item, err := client.PutBridgeSecretBinding(cmd.Context(), args[0], args[1], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeSecretBindingBundle(item))
		},
	}
	cmd.Flags().StringVar(&request.SecretRef, "secret-ref", "", "Vault secret ref")
	cmd.Flags().StringVar(&request.Kind, "kind", "", "Binding kind")
	cmd.Flags().BoolVar(
		&secretValueFromStdin,
		bridgeSecretValueStdinFlag,
		false,
		"Read the optional write-only secret value from stdin",
	)
	cmd.Flags().StringVar(
		&secretValueFile,
		bridgeSecretValueFileFlag,
		"",
		"Read the optional write-only secret value from a file",
	)
	cmd.MarkFlagsMutuallyExclusive(bridgeSecretValueStdinFlag, bridgeSecretValueFileFlag)
	mustMarkFlagRequired(cmd, "secret-ref")
	mustMarkFlagRequired(cmd, "kind")
	return cmd
}

func bridgeSecretValueInput(
	cmd *cobra.Command,
	fromStdin bool,
	filePath string,
) (*string, error) {
	switch {
	case fromStdin:
		return readBridgeSecretValue(cmd.InOrStdin(), "stdin")
	case strings.TrimSpace(filePath) != "":
		file, err := os.Open(strings.TrimSpace(filePath))
		if err != nil {
			return nil, fmt.Errorf("cli: open bridge secret value file: %w", err)
		}
		value, readErr := readBridgeSecretValue(file, "file")
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return nil, errors.Join(readErr, closeErr)
		}
		return value, nil
	default:
		return nil, nil
	}
}

func readBridgeSecretValue(reader io.Reader, source string) (*string, error) {
	if reader == nil {
		return nil, fmt.Errorf("cli: bridge secret value %s is unavailable", source)
	}
	value, err := io.ReadAll(io.LimitReader(reader, bridgeSecretValueMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("cli: read bridge secret value from %s: %w", source, err)
	}
	if len(value) > bridgeSecretValueMaxBytes {
		return nil, fmt.Errorf(
			"cli: bridge secret value from %s exceeds %d bytes",
			source,
			bridgeSecretValueMaxBytes,
		)
	}
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" {
		return nil, fmt.Errorf("cli: bridge secret value from %s cannot be empty", source)
	}
	return &trimmed, nil
}

func newBridgeSecretBindingsDeleteCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id> <binding-name>",
		Short: "Delete one bridge secret binding",
		Args:  exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if err := client.DeleteBridgeSecretBinding(cmd.Context(), args[0], args[1]); err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeSecretBindingDeleteBundle(args[0], args[1]))
		},
	}
}
