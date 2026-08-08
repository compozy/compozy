package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type gatewayProfileTransferRecord struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status string `json:"status"`
}

func newConnectExportCommand(deps commandDeps) *cobra.Command {
	var passphraseFile string
	var outputPath string
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "export <name>",
		Short: "Export a portable encrypted HTTPS profile",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			return exportGatewayProfile(cmd, deps, name, passphraseFile, outputPath, overwrite)
		},
	}
	bindGatewayPassphraseFileFlag(cmd, &passphraseFile)
	cmd.Flags().StringVar(&outputPath, "output-file", "", "Write the encrypted profile bundle to this file")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Replace an existing output file")
	mustMarkFlagRequired(cmd, "output-file")
	return cmd
}

func newConnectImportCommand(deps commandDeps) *cobra.Command {
	var passphraseFile string
	var overwrite bool
	var use bool
	cmd := &cobra.Command{
		Use:   "import <bundle>",
		Short: "Import a portable encrypted HTTPS profile",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return importGatewayProfile(cmd, deps, strings.TrimSpace(args[0]), passphraseFile, overwrite, use)
		},
	}
	bindGatewayPassphraseFileFlag(cmd, &passphraseFile)
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Replace an existing profile with the bundle profile")
	cmd.Flags().BoolVar(&use, "use", false, "Make the imported profile active")
	return cmd
}

func bindGatewayPassphraseFileFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "passphrase-file", "", "Read the bundle passphrase from a private file")
	mustMarkFlagRequired(cmd, "passphrase-file")
}

func exportGatewayProfile(
	cmd *cobra.Command,
	deps commandDeps,
	name string,
	passphrasePath string,
	outputPath string,
	overwrite bool,
) (returnErr error) {
	homePaths, err := deps.resolveHome()
	if err != nil {
		return err
	}
	lock, gatewayConfig, err := acquireRecoveredGatewayProfileState(cmd.Context(), homePaths, deps)
	if err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, lock.Release())
	}()
	profile, exists := findGatewayProfile(gatewayConfig.Connections, name)
	if !exists {
		return fmt.Errorf("cli: gateway profile %q was not found", name)
	}
	if profile.Scheme != gatewaySchemeHTTPS {
		return errors.New("cli: only HTTPS gateway profiles can be exported")
	}
	if err := ensureGatewayProfileTransactionReady(homePaths.GatewayCredentialsDir, name); err != nil {
		return err
	}
	credential, err := deps.readGatewayCredential(homePaths.GatewayCredentialsDir, name)
	if err != nil {
		return fmt.Errorf("cli: read gateway profile credential for export: %w", err)
	}
	passphrase, err := readGatewayProfilePassphrase(passphrasePath)
	if err != nil {
		return err
	}
	payload, err := encryptGatewayProfileBundle(gatewayProfileBundle{
		Profile: profile, Credential: credential,
	}, passphrase, nil)
	if err != nil {
		return err
	}
	path, err := filepath.Abs(strings.TrimSpace(outputPath))
	if err != nil {
		return fmt.Errorf("cli: resolve gateway profile bundle output: %w", err)
	}
	if err := writePrivateNewFile(path, payload, overwrite); err != nil {
		return err
	}
	return writeCommandOutput(cmd, gatewayProfileTransferOutput(gatewayProfileTransferRecord{
		Name: profile.Name, Path: path, Status: "exported",
	}))
}

func importGatewayProfile(
	cmd *cobra.Command,
	deps commandDeps,
	bundlePath string,
	passphrasePath string,
	overwrite bool,
	use bool,
) error {
	payload, err := readPrivateBoundedFile(bundlePath, gatewayProfileBundleMaxBytes, "gateway profile bundle")
	if err != nil {
		return err
	}
	passphrase, err := readGatewayProfilePassphrase(passphrasePath)
	if err != nil {
		return err
	}
	bundle, err := decryptGatewayProfileBundle(payload, passphrase)
	if err != nil {
		return err
	}
	homePaths, err := deps.resolveHome()
	if err != nil {
		return err
	}
	if err := persistPairedGatewayProfile(
		cmd.Context(), homePaths, bundle.Profile, bundle.Credential, overwrite, use, deps,
	); err != nil {
		return err
	}
	path, err := filepath.Abs(bundlePath)
	if err != nil {
		return fmt.Errorf("cli: resolve imported gateway profile bundle path: %w", err)
	}
	return writeCommandOutput(cmd, gatewayProfileTransferOutput(gatewayProfileTransferRecord{
		Name: bundle.Profile.Name, Path: path, Status: "imported",
	}))
}

func readGatewayProfilePassphrase(path string) ([]byte, error) {
	payload, err := readPrivateBoundedFile(path, gatewayProfilePassphraseMax, "gateway profile passphrase file")
	if err != nil {
		return nil, err
	}
	passphrase := []byte(strings.TrimRight(string(payload), "\r\n"))
	if err := validateGatewayProfilePassphrase(passphrase); err != nil {
		return nil, err
	}
	return passphrase, nil
}

func gatewayProfileTransferOutput(record gatewayProfileTransferRecord) outputBundle {
	return simpleGatewayOutput(record, func() string {
		return renderHumanSection("Connection Transfer", []keyValue{
			{Label: cliNameValue, Value: record.Name},
			{Label: automationPathValue, Value: record.Path},
			{Label: automationStatusValue, Value: record.Status},
		})
	}, func() string {
		return renderToonObject(
			"connection_transfer",
			[]string{automationNameKey, automationPathKey, automationStatusKey},
			[]string{record.Name, record.Path, record.Status},
		)
	})
}
