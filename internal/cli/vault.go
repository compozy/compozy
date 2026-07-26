package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	vaultRefValue = "Ref"
	vaultRefKey   = "ref"
)

const (
	vaultCreatedValue = "Created"
	vaultKindValue    = "Kind"
	vaultStatusValue  = "Status"
	vaultCreatedAtKey = "created_at"
	vaultDeletedKey   = "deleted"
	vaultKindKey      = "kind"
	vaultListKey      = "list"
	vaultStatusKey    = "status"
	vaultVaultKey     = "vault"
)

type vaultDeleteRecord struct {
	Ref    string `json:"ref"`
	Status string `json:"status"`
}

func newVaultCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   vaultVaultKey,
		Short: "Manage encrypted daemon vault metadata and write-only secrets",
	}
	cmd.AddCommand(newVaultListCommand(deps))
	cmd.AddCommand(newVaultGetCommand(deps))
	cmd.AddCommand(newVaultPutCommand(deps))
	cmd.AddCommand(newVaultDeleteCommand(deps))
	return cmd
}

func newVaultListCommand(deps commandDeps) *cobra.Command {
	var prefix string
	var namespace string

	cmd := &cobra.Command{
		Use:   vaultListKey,
		Short: "List redacted vault secret metadata",
		Example: `  # List session-scoped vault entries
  agh vault list --prefix vault:sessions/sess_123/

  # List all provider vault entries as JSON
  agh vault list --namespace providers -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			items, err := client.ListVaultSecrets(cmd.Context(), VaultListQuery{
				Prefix:    strings.TrimSpace(prefix),
				Namespace: strings.TrimSpace(namespace),
			})
			if err != nil {
				return err
			}
			return writeVaultRecordsOutput(cmd, items, deps.now)
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "Vault ref prefix, for example vault:sessions/<session_id>/")
	cmd.Flags().StringVar(&namespace, "namespace", "", "Vault namespace filter")
	return cmd
}

func newVaultGetCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "get <ref>",
		Short: "Show redacted metadata for one vault secret",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			item, err := client.GetVaultSecret(cmd.Context(), strings.TrimSpace(args[0]))
			if err != nil {
				return err
			}
			return writeVaultRecordOutput(cmd, item)
		},
	}
}

func newVaultPutCommand(deps commandDeps) *cobra.Command {
	var kind string
	var valueStdin bool

	cmd := &cobra.Command{
		Use:   "put <ref>",
		Short: "Store one write-only vault secret from stdin",
		Example: `  # Store a session vault secret
  printf "%s" "$TOKEN" | agh vault put vault:sessions/sess_123/github-token --kind token --value-stdin

  # Rotate a value without changing its existing kind metadata
  printf "%s" "$TOKEN" | agh vault put vault:sessions/sess_123/github-token --value-stdin`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !valueStdin {
				return errors.New("cli: --value-stdin is required")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			value, err := readVaultSecretStdin(cmd)
			if err != nil {
				return err
			}

			item, err := client.PutVaultSecret(cmd.Context(), PutVaultSecretRequest{
				Ref:         strings.TrimSpace(args[0]),
				Kind:        strings.TrimSpace(kind),
				SecretValue: value,
			})
			if err != nil {
				return err
			}
			return writeVaultRecordOutput(cmd, item)
		},
	}
	cmd.Flags().StringVar(&kind, vaultKindKey, "", "Optional secret kind metadata")
	cmd.Flags().BoolVar(&valueStdin, "value-stdin", false, "Read the secret value from stdin")
	return cmd
}

func newVaultDeleteCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <ref>",
		Short: "Delete one vault secret",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			ref := strings.TrimSpace(args[0])
			if err := client.DeleteVaultSecret(cmd.Context(), ref); err != nil {
				return err
			}
			return writeVaultDeleteOutput(cmd, vaultDeleteRecord{Ref: ref, Status: vaultDeletedKey})
		},
	}
}

func readVaultSecretStdin(cmd *cobra.Command) (string, error) {
	content, err := readOptionalCommandInput(cmd.InOrStdin())
	if err != nil {
		return "", err
	}
	content = strings.TrimRight(content, "\r\n")
	if strings.TrimSpace(content) == "" {
		return "", errors.New("cli: secret value is required via --value-stdin")
	}
	return content, nil
}

func writeVaultRecordsOutput(cmd *cobra.Command, items []VaultRecord, now func() time.Time) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode == OutputJSONL {
		for _, item := range items {
			if err := writeJSONLine(cmd, item); err != nil {
				return err
			}
		}
		return nil
	}
	return writeCommandOutput(cmd, vaultRecordsBundle(items, now))
}

func writeVaultRecordOutput(cmd *cobra.Command, item VaultRecord) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode == OutputJSONL {
		return writeJSONLine(cmd, item)
	}
	return writeCommandOutput(cmd, vaultRecordBundle(item))
}

func writeVaultDeleteOutput(cmd *cobra.Command, item vaultDeleteRecord) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode == OutputJSONL {
		return writeJSONLine(cmd, item)
	}
	return writeCommandOutput(cmd, vaultDeleteBundle(item))
}

func vaultRecordsBundle(items []VaultRecord, now func() time.Time) outputBundle {
	return listBundle[VaultRecord](
		items,
		items,
		"Vault Secrets",
		[]string{vaultRefValue, "Namespace", vaultKindValue, authoredContextPresentValue, authoredContextUpdatedValue},
		"vault_secrets",
		[]string{vaultRefKey, "namespace", vaultKindKey, providerAuthStatePresent, bridgeUpdatedAtKey},
		func(item VaultRecord) []string {
			return []string{
				item.Ref,
				item.Namespace,
				stringOrDash(item.Kind),
				fmt.Sprintf("%t", item.Present),
				formatAge(now, item.UpdatedAt),
			}
		},
		func(item VaultRecord) []string {
			return []string{
				item.Ref,
				item.Namespace,
				item.Kind,
				fmt.Sprintf("%t", item.Present),
				formatTime(item.UpdatedAt),
			}
		},
	)
}

func vaultRecordBundle(item VaultRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Vault Secret", []keyValue{
				{Label: vaultRefValue, Value: item.Ref},
				{Label: "Namespace", Value: stringOrDash(item.Namespace)},
				{Label: vaultKindValue, Value: stringOrDash(item.Kind)},
				{Label: authoredContextPresentValue, Value: fmt.Sprintf("%t", item.Present)},
				{Label: vaultCreatedValue, Value: stringOrDash(formatTime(item.CreatedAt))},
				{Label: authoredContextUpdatedValue, Value: stringOrDash(formatTime(item.UpdatedAt))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"vault_secret",
				[]string{
					vaultRefKey,
					"namespace",
					vaultKindKey,
					providerAuthStatePresent,
					vaultCreatedAtKey,
					bridgeUpdatedAtKey,
				},
				[]string{
					item.Ref,
					item.Namespace,
					item.Kind,
					fmt.Sprintf("%t", item.Present),
					formatTime(item.CreatedAt),
					formatTime(item.UpdatedAt),
				},
			), nil
		},
	}
}

func vaultDeleteBundle(item vaultDeleteRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Vault Secret", []keyValue{
				{Label: vaultRefValue, Value: item.Ref},
				{Label: vaultStatusValue, Value: item.Status},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"vault_secret",
				[]string{vaultRefKey, vaultStatusKey},
				[]string{item.Ref, item.Status},
			), nil
		},
	}
}
