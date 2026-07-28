package cli

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"strings"

	"golang.org/x/term"
)

const bridgeSetupGeneratedSecretBytes = 32

type bridgeSetupWizardRunner func(
	context.Context,
	bridgeSetupWizardInput,
) (bridgeSetupWizardSelection, error)

type bridgeSetupSecretGenerator func() (string, error)

type bridgeSetupPrompt struct {
	Key      string
	Label    string
	Secret   bool
	Required bool
	Validate func(string) error
}

type bridgeSetupWizardInput struct {
	Platform string
	Prompts  []bridgeSetupPrompt
	Defaults map[string]string
	Input    io.Reader
	Output   io.Writer
}

type bridgeSetupWizardSelection struct {
	Values map[string]string
}

func runBridgeSetupWizard(
	ctx context.Context,
	input bridgeSetupWizardInput,
) (bridgeSetupWizardSelection, error) {
	if input.Input == nil || input.Output == nil {
		return bridgeSetupWizardSelection{}, errors.New("cli: bridge setup wizard input and output are required")
	}
	values := make(map[string]string, len(input.Defaults)+len(input.Prompts))
	maps.Copy(values, input.Defaults)
	reader := bufio.NewReader(input.Input)
	for _, prompt := range input.Prompts {
		value, err := readBridgeSetupWizardPrompt(ctx, reader, input, prompt)
		if err != nil {
			return bridgeSetupWizardSelection{}, err
		}
		values[prompt.Key] = value
	}
	return bridgeSetupWizardSelection{Values: values}, nil
}

func readBridgeSetupWizardPrompt(
	ctx context.Context,
	reader *bufio.Reader,
	input bridgeSetupWizardInput,
	prompt bridgeSetupPrompt,
) (string, error) {
	for {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("cli: bridge setup canceled: %w", err)
		}
		defaultValue := strings.TrimSpace(input.Defaults[prompt.Key])
		if err := writeBridgeSetupPrompt(input.Output, prompt, defaultValue); err != nil {
			return "", err
		}
		value, exhausted, err := readBridgeSetupPromptValue(reader, input.Input, input.Output, prompt.Secret)
		if err != nil {
			return "", fmt.Errorf("cli: read %s: %w", prompt.Key, err)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			value = defaultValue
		}
		validationErr := validateBridgeSetupPromptValue(prompt, value)
		if validationErr == nil {
			return value, nil
		}
		if exhausted {
			return "", fmt.Errorf("cli: invalid %s: %w", prompt.Key, validationErr)
		}
		if _, err := fmt.Fprintf(input.Output, "Invalid value: %s\n", validationErr); err != nil {
			return "", fmt.Errorf("cli: write %s validation: %w", prompt.Key, err)
		}
	}
}

func writeBridgeSetupPrompt(output io.Writer, prompt bridgeSetupPrompt, defaultValue string) error {
	suffix := ""
	if defaultValue != "" {
		suffix = " [" + defaultValue + "]"
	}
	if _, err := fmt.Fprintf(output, "%s%s: ", prompt.Label, suffix); err != nil {
		return fmt.Errorf("cli: write %s prompt: %w", prompt.Key, err)
	}
	return nil
}

func readBridgeSetupPromptValue(
	reader *bufio.Reader,
	input io.Reader,
	output io.Writer,
	secret bool,
) (string, bool, error) {
	file, terminalInput := input.(*os.File)
	terminalFD := -1
	if terminalInput {
		terminalFD = int(file.Fd())
	}
	if secret && terminalInput && term.IsTerminal(terminalFD) {
		value, err := term.ReadPassword(terminalFD)
		if _, writeErr := fmt.Fprintln(output); writeErr != nil {
			return "", false, fmt.Errorf("write hidden-input newline: %w", writeErr)
		}
		if err != nil {
			return "", false, err
		}
		return string(value), false, nil
	}

	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", false, err
	}
	return line, errors.Is(err, io.EOF), nil
}

func validateBridgeSetupPromptValue(prompt bridgeSetupPrompt, value string) error {
	if value == bridgeSetupMaskedSecret && prompt.Secret {
		return nil
	}
	if prompt.Required && strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", prompt.Label)
	}
	if prompt.Validate != nil && strings.TrimSpace(value) != "" {
		return prompt.Validate(value)
	}
	return nil
}

func generateBridgeSetupSecret() (string, error) {
	raw := make([]byte, bridgeSetupGeneratedSecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("cli: generate bridge setup secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
