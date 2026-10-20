package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/vault"
	"github.com/gofrs/flock"
	"github.com/spf13/cobra"
)

type vaultKeyMigrationResult struct {
	Provider        string   `json:"provider"`
	VerifiedRecords int      `json:"verified_records"`
	FileKeyRetained bool     `json:"file_key_retained"`
	Warnings        []string `json:"warnings,omitempty"`
}

func newVaultKeyCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "key", Short: "Manage the local Vault encryption key provider"}
	var removeFile bool
	migrate := &cobra.Command{
		Use:   "migrate",
		Short: "Explicitly migrate the local Vault file key to the OS keyring",
		Long: "Import the existing local vault.key, verify every encrypted record, then select os-keyring. " +
			"This command can prompt for OS credential-store permission. It operates on the local Compozy home.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := migrateVaultKey(cmd.Context(), deps, removeFile)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{
				jsonValue: result,
				human: func() (string, error) {
					return fmt.Sprintf(
						"Vault provider: %s\nVerified records: %d\nFile key retained: %t\n%s\n",
						result.Provider,
						result.VerifiedRecords,
						result.FileKeyRetained,
						strings.Join(result.Warnings, "\n"),
					), nil
				},
			})
		},
	}
	migrate.Flags().BoolVar(&removeFile, "remove-file-key", false, "Remove vault.key after verified migration")
	cmd.AddCommand(migrate)
	return cmd
}

func migrateVaultKey(
	ctx context.Context,
	deps commandDeps,
	removeFile bool,
) (result vaultKeyMigrationResult, err error) {
	if strings.TrimSpace(deps.getenv("COMPOZY_VAULT_KEY")) != "" {
		return result, errors.New(
			"cli: unset COMPOZY_VAULT_KEY before migrating; migration imports the existing vault.key",
		)
	}
	home, err := deps.resolveHome()
	if err != nil {
		return result, err
	}
	lock := flock.New(filepath.Join(home.HomeDir, ".vault-key-migration.lock"), flock.SetPermissions(0o600))
	locked, err := lock.TryLockContext(ctx, 10*time.Millisecond)
	if err != nil {
		return result, fmt.Errorf("cli: lock vault key migration: %w", err)
	}
	if !locked {
		return result, errors.New("cli: vault migration lock unavailable")
	}
	defer func() { recordVaultMigrationCleanup(&result, &err, lock.Unlock(), "unlock migration") }()
	db, err := openLocalGlobalDatabase(ctx, home.DatabaseFile, "vault key migration")
	if err != nil {
		return result, err
	}
	defer func() { recordVaultMigrationCleanup(&result, &err, db.Close(ctx), "close database") }()
	count, err := vault.MigrateToKeyring(ctx, home.HomeDir, db, deps.vaultKeyring, func() error {
		return activateVaultKeyring(home, removeFile, deps.removeFile, &result.Warnings)
	})
	if err != nil {
		return result, err
	}
	result.Provider = "os-keyring"
	result.VerifiedRecords = count
	_, statErr := os.Lstat(filepath.Join(home.HomeDir, "vault.key"))
	result.FileKeyRetained = statErr == nil
	if result.FileKeyRetained {
		result.Warnings = append(
			result.Warnings,
			"The plaintext key copy remains in vault.key. Repeat with --remove-file-key to remove it.",
		)
	}
	return result, nil
}

func activateVaultKeyring(
	home compozyconfig.HomePaths,
	removeFile bool,
	remove func(string) error,
	warnings *[]string,
) error {
	target, err := compozyconfig.ResolveConfigWriteTarget(home, "", compozyconfig.WriteScopeUser, "")
	if err != nil {
		return err
	}
	cfg, err := compozyconfig.EditConfigOverlayAndApply(
		home,
		"",
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			return editor.SetValue([]string{"vault", "key_provider"}, "os-keyring")
		},
		func(compozyconfig.Config) error {
			if !removeFile {
				return nil
			}
			err := remove(filepath.Join(home.HomeDir, "vault.key"))
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		},
	)
	if err != nil && cfg.Vault.KeyProvider == "os-keyring" {
		// The transaction returns its final config only after commit. A subsequent
		// directory-close error must not delete the now-authoritative keyring key.
		*warnings = append(*warnings, "Provider migration committed, but closing the config directory failed.")
		return nil
	}
	return err
}

func recordVaultMigrationCleanup(result *vaultKeyMigrationResult, primary *error, cleanup error, action string) {
	if cleanup == nil {
		return
	}
	if result.Provider == "os-keyring" {
		result.Warnings = append(result.Warnings, "Migration committed, but could not "+action+".")
		return
	}
	*primary = errors.Join(*primary, fmt.Errorf("cli: %s: %w", action, cleanup))
}
