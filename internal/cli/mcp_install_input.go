package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"golang.org/x/term"
)

const (
	mcpValueInputFlag = "set"
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

func mcpInstallInputs(
	setValues []string,
	secretIDs []string,
	vaultRefs []string,
	readSecret func(string) (string, error),
) (map[string]contract.SettingsMCPSecretInputPayload, error) {
	inputs := make(map[string]contract.SettingsMCPSecretInputPayload, len(setValues)+len(secretIDs)+len(vaultRefs))
	for _, assignment := range setValues {
		key, value, err := parseMCPInstallAssignment("--set", assignment)
		if err != nil {
			return nil, err
		}
		if _, exists := inputs[key]; exists {
			return nil, fmt.Errorf("cli: MCP install field %q is assigned more than once", key)
		}
		inputs[key] = contract.SettingsMCPSecretInputPayload{Value: value}
	}
	for _, rawID := range secretIDs {
		key, err := parseMCPInstallInputID("--secret", rawID)
		if err != nil {
			return nil, err
		}
		if _, exists := inputs[key]; exists {
			return nil, fmt.Errorf("cli: MCP install field %q is assigned more than once", key)
		}
		value, err := readSecret(key)
		if err != nil {
			return nil, err
		}
		inputs[key] = contract.SettingsMCPSecretInputPayload{Value: value}
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
	if len(inputs) == 0 {
		return nil, nil
	}
	return inputs, nil
}

func parseMCPInstallInputID(flag string, rawID string) (string, error) {
	id := strings.TrimSpace(rawID)
	if id == "" || strings.Contains(id, "=") {
		return "", fmt.Errorf("cli: %s requires an input ID", flag)
	}
	return id, nil
}

func parseMCPInstallAssignment(flag string, assignment string) (string, string, error) {
	key, value, ok := strings.Cut(assignment, "=")
	key = strings.TrimSpace(key)
	if !ok || key == "" || strings.TrimSpace(value) == "" {
		return "", "", fmt.Errorf("cli: %s requires KEY=VALUE", flag)
	}
	id, err := parseMCPInstallInputID(flag, key)
	if err != nil {
		return "", "", err
	}
	return id, strings.TrimSpace(value), nil
}

func validateMCPInstallScope(scope contract.SettingsWorkspaceScopeKind, workspaceID string) error {
	switch scope {
	case "":
		if strings.TrimSpace(workspaceID) != "" {
			return errors.New("cli: --workspace requires --scope workspace")
		}
		return nil
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
