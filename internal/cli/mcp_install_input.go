package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"golang.org/x/term"
)

const (
	mcpValueInputFlag         = "set"
	mcpOAuthValueInputFlag    = "oauth-client-secret"
	mcpOAuthVaultRefInputFlag = "oauth-client-secret-vault-ref"
)

type mcpInstallSecretReader struct {
	input        io.Reader
	output       io.Writer
	buffered     *bufio.Reader
	isTerminal   func(io.Reader) bool
	readPassword func(int) ([]byte, error)
}

func newMCPInstallSecretReader(
	input io.Reader,
	output io.Writer,
	isTerminal func(io.Reader) bool,
) *mcpInstallSecretReader {
	return &mcpInstallSecretReader{
		input:        input,
		output:       output,
		buffered:     bufio.NewReader(input),
		isTerminal:   isTerminal,
		readPassword: term.ReadPassword,
	}
}

func (r *mcpInstallSecretReader) Read(label string) (string, error) {
	if r.isTerminal(r.input) {
		return r.readHidden(label)
	}
	value, err := r.buffered.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("cli: read MCP install field %q from stdin: %w", label, err)
	}
	return validateMCPInstallSecretValue(label, value)
}

func (r *mcpInstallSecretReader) readHidden(label string) (string, error) {
	file, ok := r.input.(*os.File)
	if !ok {
		return "", errors.New("cli: terminal MCP install input must be an operating-system file")
	}
	if _, err := fmt.Fprintf(r.output, "%s: ", label); err != nil {
		return "", fmt.Errorf("cli: write MCP install prompt: %w", err)
	}
	value, readErr := r.readPassword(int(file.Fd()))
	_, newlineErr := fmt.Fprintln(r.output)
	if readErr != nil {
		wrappedReadErr := fmt.Errorf("cli: read hidden MCP install field %q: %w", label, readErr)
		if newlineErr != nil {
			return "", errors.Join(wrappedReadErr, fmt.Errorf("cli: write hidden-input newline: %w", newlineErr))
		}
		return "", wrappedReadErr
	}
	if newlineErr != nil {
		return "", fmt.Errorf("cli: write hidden-input newline: %w", newlineErr)
	}
	return validateMCPInstallSecretValue(label, string(value))
}

func validateMCPInstallSecretValue(label string, value string) (string, error) {
	value = strings.TrimRight(value, "\r\n")
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("cli: MCP install field %q requires a non-blank value from stdin", label)
	}
	return value, nil
}

func mcpInstallOAuthClientSecretInput(
	valueSet bool,
	vaultRefSet bool,
	vaultRef string,
	readSecret func(string) (string, error),
) (*contract.SettingsMCPSecretInputPayload, error) {
	if valueSet && vaultRefSet {
		return nil, errors.New("cli: OAuth client secret is assigned more than once")
	}
	if valueSet {
		value, err := readSecret("OAuth client secret")
		if err != nil {
			return nil, err
		}
		return &contract.SettingsMCPSecretInputPayload{Value: value}, nil
	}
	if vaultRefSet {
		trimmedRef := strings.TrimSpace(vaultRef)
		if trimmedRef == "" {
			return nil, errors.New("cli: --oauth-client-secret-vault-ref requires a non-blank value")
		}
		return &contract.SettingsMCPSecretInputPayload{VaultRef: trimmedRef}, nil
	}
	return nil, nil
}

func mcpInstallEnvInputs(
	valueKeys []string,
	vaultRefs []string,
	readSecret func(string) (string, error),
) (map[string]contract.SettingsMCPSecretInputPayload, error) {
	inputs := make(map[string]contract.SettingsMCPSecretInputPayload, len(valueKeys)+len(vaultRefs))
	pendingValues := make([]string, 0, len(valueKeys))
	for _, rawKey := range valueKeys {
		key, err := parseMCPInstallValueKey(rawKey)
		if err != nil {
			return nil, err
		}
		if _, exists := inputs[key]; exists {
			return nil, fmt.Errorf("cli: MCP install field %q is assigned more than once", key)
		}
		inputs[key] = contract.SettingsMCPSecretInputPayload{}
		pendingValues = append(pendingValues, key)
	}
	for _, assignment := range vaultRefs {
		key, ref, err := parseMCPInstallAssignment("--vault-ref", assignment)
		if err != nil {
			return nil, err
		}
		if _, exists := inputs[key]; exists {
			return nil, fmt.Errorf("cli: MCP install field %q is assigned more than once", key)
		}
		inputs[key] = contract.SettingsMCPSecretInputPayload{VaultRef: strings.TrimSpace(ref)}
	}
	for _, key := range pendingValues {
		value, err := readSecret(key)
		if err != nil {
			return nil, err
		}
		inputs[key] = contract.SettingsMCPSecretInputPayload{Value: value}
	}
	if len(inputs) == 0 {
		return nil, nil
	}
	return inputs, nil
}

func parseMCPInstallValueKey(rawKey string) (string, error) {
	key := strings.TrimSpace(rawKey)
	if key == "" || strings.Contains(key, "=") {
		return "", errors.New("cli: --set requires a field name without a value")
	}
	if err := aghconfig.ValidateMCPStdioEnvName("--set", key, true); err != nil {
		return "", fmt.Errorf("cli: invalid MCP install field: %w", err)
	}
	return key, nil
}

func parseMCPInstallAssignment(flag string, assignment string) (string, string, error) {
	key, value, ok := strings.Cut(assignment, "=")
	key = strings.TrimSpace(key)
	if !ok || key == "" || strings.TrimSpace(value) == "" {
		return "", "", fmt.Errorf("cli: %s requires KEY=VALUE", flag)
	}
	if err := aghconfig.ValidateMCPStdioEnvName(flag, key, true); err != nil {
		return "", "", fmt.Errorf("cli: invalid MCP install field: %w", err)
	}
	return key, value, nil
}

func validateMCPInstallScope(scope contract.SettingsWorkspaceScopeKind, workspaceID string) error {
	switch scope {
	case contract.SettingsWorkspaceScopeGlobal:
		if strings.TrimSpace(workspaceID) != "" {
			return errors.New("cli: --workspace requires --scope workspace")
		}
	case contract.SettingsWorkspaceScopeWorkspace:
		if strings.TrimSpace(workspaceID) == "" {
			return errors.New("cli: --scope workspace requires --workspace")
		}
	default:
		return fmt.Errorf("cli: unsupported MCP install scope %q", scope)
	}
	return nil
}
