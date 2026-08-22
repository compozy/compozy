package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/vault"
	"github.com/spf13/cobra"
)

const secretValueStdinFlag = "value-stdin"

type secretMutationRecord struct {
	Ref      string   `json:"ref"`
	Profile  string   `json:"profile"`
	Status   string   `json:"status"`
	Warnings []string `json:"warnings,omitempty"`
}

func newSecretCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "secret", Short: "Manage credentials for the active profile"}
	cmd.AddCommand(newSecretSetCommand(deps), newSecretRemoveCommand(deps))
	return cmd
}

func newSecretSetCommand(deps commandDeps) *cobra.Command {
	var valueStdin bool
	var fromEnv string
	cmd := &cobra.Command{
		Use:   "set <providers/<provider>/<slot>|extensions/<extension>/<key>>",
		Short: "Set one write-only credential for the active profile",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolution, client, err := resolveSecretProfile(cmd, deps)
			if err != nil {
				return err
			}
			ref, err := secretRefForProfile(resolution.Profile.Name, args[0])
			if err != nil {
				return err
			}
			value, err := readProfileSecretValue(cmd, deps, resolution.Profile.Name, fromEnv, valueStdin)
			if err != nil {
				return err
			}
			metadata, err := client.PutVaultSecret(cmd.Context(), PutVaultSecretRequest{
				Ref: ref, Kind: secretKindFromPath(args[0]), SecretValue: value,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, secretMutationBundle(secretMutationRecord{
				Ref: metadata.Ref, Profile: resolution.Profile.Name, Status: "saved",
			}))
		},
	}
	cmd.Flags().BoolVar(&valueStdin, secretValueStdinFlag, false, "Read the secret value from stdin")
	cmd.Flags().StringVar(&fromEnv, "from-env", "", "Import the value from an environment variable (user scope only)")
	return cmd
}

func newSecretRemoveCommand(deps commandDeps) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "rm <providers/<provider>/<slot>|extensions/<extension>/<key>>",
		Aliases: []string{"remove"},
		Short:   "Remove one credential override from the active profile",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolution, client, err := resolveSecretProfile(cmd, deps)
			if err != nil {
				return err
			}
			ref, err := secretRefForProfile(resolution.Profile.Name, args[0])
			if err != nil {
				return err
			}
			if err := confirmProfileSecretRemoval(cmd, deps, resolution.Profile, args[0], yes); err != nil {
				return err
			}
			if err := client.DeleteVaultSecret(cmd.Context(), ref); err != nil {
				return err
			}
			return writeCommandOutput(cmd, secretMutationBundle(secretMutationRecord{
				Ref: ref, Profile: resolution.Profile.Name, Status: "removed",
				Warnings: profileSecretRemovalWarnings(resolution.Profile, args[0]),
			}))
		},
	}
	cmd.Flags().BoolVarP(&yes, yesFlagName, "y", false, "Confirm removal without prompting")
	return cmd
}

func resolveSecretProfile(
	cmd *cobra.Command,
	deps commandDeps,
) (profileResolution, DaemonClient, error) {
	profiles, client, err := profileClientFromDeps(deps)
	if err != nil {
		return profileResolution{}, nil, err
	}
	resolution, err := resolveCommandProfile(cmd.Context(), cmd, deps, profiles, client)
	if err != nil {
		return profileResolution{}, nil, err
	}
	return resolution, client, nil
}

func secretRefForProfile(profileName, path string) (string, error) {
	path = strings.Trim(strings.TrimSpace(path), "/")
	segments := strings.Split(path, "/")
	if len(segments) != 3 || (segments[0] != "providers" && segments[0] != "extensions") {
		return "", errors.New("cli: secret path must match providers/<provider>/<slot> or extensions/<extension>/<key>")
	}
	profileName = strings.TrimSpace(profileName)
	if profileName == "default" {
		ref := "vault:" + path
		if err := vault.ValidateSecretRef(ref); err != nil {
			return "", err
		}
		return ref, nil
	}
	prefix, err := vault.ProfileSecretOwnerPrefix(profileName, segments[0], segments[1])
	if err != nil {
		return "", err
	}
	ref := prefix + segments[2]
	if err := vault.ValidateProfileSecretRefAccess(ref, profileName); err != nil {
		return "", err
	}
	return ref, nil
}

func readProfileSecretValue(
	cmd *cobra.Command,
	deps commandDeps,
	profileName string,
	fromEnv string,
	valueStdin bool,
) (string, error) {
	fromEnv = strings.TrimSpace(fromEnv)
	if fromEnv != "" && valueStdin {
		return "", errors.New("cli: --from-env and --value-stdin are mutually exclusive")
	}
	if fromEnv != "" {
		if profileName != "default" {
			typed := &vault.ProfileSecretError{
				Code:    "profile_secret_env_forbidden",
				Message: "profile secrets must live in the vault — the process environment is shared by every profile",
				Action:  "pipe the value to compozy secret set --value-stdin",
				Cause:   vault.ErrProfileSecretEnvForbidden,
			}
			return "", newProfileSelectionError(typed.Code, typed.Message, typed.Action)
		}
		value := deps.getenv(fromEnv)
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("cli: environment variable %q is empty or unset", fromEnv)
		}
		return value, nil
	}
	if valueStdin {
		return readVaultSecretStdin(cmd)
	}
	return newExtensionSecretReader(cmd.InOrStdin(), cmd.ErrOrStderr(), deps.inputIsTerminal).Read("Enter value")
}

func confirmProfileSecretRemoval(
	cmd *cobra.Command,
	deps commandDeps,
	profile contract.Profile,
	path string,
	yes bool,
) error {
	if yes || profile.WorkItems == 0 {
		return nil
	}
	mode, err := resolveInheritedOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode != OutputHuman || deps.inputIsTerminal == nil || !deps.inputIsTerminal(cmd.InOrStdin()) {
		return errors.New("cli: secret removal with owned work requires --yes")
	}
	provider := strings.TrimSpace(strings.Split(strings.Trim(path, "/"), "/")[1])
	if _, err := fmt.Fprintf(
		cmd.ErrOrStderr(),
		"%s has %d work items that may use %s — future runs fall back to the user credential. Remove? [y/N] ",
		profile.Name,
		profile.WorkItems,
		provider,
	); err != nil {
		return fmt.Errorf("cli: write secret removal confirmation: %w", err)
	}
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("cli: read secret removal confirmation: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		return errors.New("cli: secret removal declined")
	}
	return nil
}

func profileSecretRemovalWarnings(profile contract.Profile, path string) []string {
	if profile.WorkItems == 0 {
		return nil
	}
	segments := strings.Split(strings.Trim(strings.TrimSpace(path), "/"), "/")
	provider := "credential"
	if len(segments) > 1 && strings.TrimSpace(segments[1]) != "" {
		provider = strings.TrimSpace(segments[1])
	}
	return []string{fmt.Sprintf(
		"%s has %d work items using %s — future runs fall back to the user key",
		profile.Name,
		profile.WorkItems,
		provider,
	)}
}

func secretKindFromPath(path string) string {
	segments := strings.Split(strings.Trim(strings.TrimSpace(path), "/"), "/")
	if len(segments) != 3 {
		return ""
	}
	return segments[2]
}

func secretMutationBundle(record secretMutationRecord) outputBundle {
	rows := []keyValue{
		{Label: vaultRefValue, Value: record.Ref},
		{Label: "Profile", Value: record.Profile},
		{Label: vaultStatusValue, Value: record.Status},
	}
	if len(record.Warnings) > 0 {
		rows = append(rows, keyValue{Label: "Warning", Value: strings.Join(record.Warnings, "; ")})
	}
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Profile Secret", rows), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"profile_secret",
				[]string{"ref", "profile", "status", "warnings"},
				[]string{record.Ref, record.Profile, record.Status, strings.Join(record.Warnings, "; ")},
			), nil
		},
	}
}
