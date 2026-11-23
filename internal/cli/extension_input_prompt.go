package cli

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	extensionInputTypeSecret  = "secret"
	extensionInputTypeBoolean = "boolean"
)

var errExtensionInputPromptAborted = errors.New("cli: extension input prompt aborted; nothing was written")

// extensionPromptInput is one input the CLI asks for. State is empty for install refusals, which list
// only inputs that need a value. Subprocess reports that the extension process consumes the input.
type extensionPromptInput struct {
	ID         string
	Prompt     string
	Type       string
	Required   bool
	Default    json.RawMessage
	State      string
	Subprocess bool
}

// extensionInputPrompter asks for extension input values on an interactive terminal. Install uses it on
// extension_inputs_required and setup uses it for the inputs its mode selects.
type extensionInputPrompter struct {
	input        io.Reader
	output       io.Writer
	reader       *bufio.Reader
	readPassword func(int) ([]byte, error)
}

func newExtensionInputPrompter(cmd *cobra.Command) *extensionInputPrompter {
	return &extensionInputPrompter{
		input:        cmd.InOrStdin(),
		output:       cmd.ErrOrStderr(),
		reader:       bufio.NewReader(cmd.InOrStdin()),
		readPassword: term.ReadPassword,
	}
}

// extensionInputsInteractive reports whether the CLI may prompt: human output with terminal stdin and stdout.
func extensionInputsInteractive(cmd *cobra.Command, deps commandDeps) (bool, error) {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return false, err
	}
	return mode == OutputHuman &&
		deps.inputIsTerminal != nil && deps.inputIsTerminal(cmd.InOrStdin()) &&
		deps.outputIsTerminal != nil && deps.outputIsTerminal(cmd.OutOrStdout()), nil
}

func writerIsTerminal(output io.Writer) bool {
	file, ok := output.(*os.File)
	return ok && supportBundleInputIsTerminal(file)
}

// promptAll asks for each input in order and returns only the values the operator entered.
func (p *extensionInputPrompter) promptAll(
	ctx context.Context, inputs []extensionPromptInput,
) (map[string]extensioninput.Value, error) {
	values := make(map[string]extensioninput.Value, len(inputs))
	for _, input := range inputs {
		value, entered, err := p.promptOne(ctx, input)
		if err != nil {
			return nil, err
		}
		if entered {
			values[input.ID] = extensioninput.Value{Value: value}
		}
	}
	return values, nil
}

func (p *extensionInputPrompter) promptOne(
	ctx context.Context, input extensionPromptInput,
) (json.RawMessage, bool, error) {
	label := extensionInputPromptLabel(input)
	for {
		if _, err := fmt.Fprint(p.output, label); err != nil {
			return nil, false, fmt.Errorf("cli: write extension input prompt: %w", err)
		}
		answer, err := p.read(ctx, input.Type == extensionInputTypeSecret)
		if err != nil {
			return nil, false, err
		}
		value, entered, reason := extensionPromptAnswer(input, answer)
		if reason == "" {
			if !entered && input.State == contract.ExtensionInputStateDaemonEnv {
				if _, err := fmt.Fprintf(
					p.output, "%s: still using the daemon environment; %s\n",
					extensionInputPromptText(input), extensionDaemonEnvNote(input),
				); err != nil {
					return nil, false, fmt.Errorf("cli: write extension input notice: %w", err)
				}
			}
			return value, entered, nil
		}
		if _, err := fmt.Fprintf(p.output, "  %s\n", reason); err != nil {
			return nil, false, fmt.Errorf("cli: write extension input reason: %w", err)
		}
	}
}

// extensionDaemonEnvNote names who misses a daemon_env value: only the extension process lacks it; an
// MCP server reads the daemon environment until a stored value replaces it.
func extensionDaemonEnvNote(input extensionPromptInput) string {
	if input.Subprocess {
		return "the extension process does not receive it."
	}
	return "a value saved here is stored for the MCP server."
}

// extensionPromptAnswer maps one answer to a typed value. An empty answer keeps a set value, the daemon
// environment, a default, or an optional input unchanged. A non-empty reason asks again.
func extensionPromptAnswer(input extensionPromptInput, answer string) (json.RawMessage, bool, string) {
	if input.Type != extensionInputTypeSecret {
		answer = strings.TrimSpace(answer)
	}
	if strings.TrimSpace(answer) == "" {
		switch {
		case input.State == contract.ExtensionInputStateSet,
			input.State == contract.ExtensionInputStateDaemonEnv,
			len(input.Default) > 0,
			!input.Required:
			return nil, false, ""
		case input.Type == extensionInputTypeBoolean:
			return json.RawMessage("false"), true, ""
		default:
			return nil, false, "A value is required."
		}
	}
	raw, err := json.Marshal(answer)
	if err != nil {
		return nil, false, "The value could not be encoded."
	}
	if input.Type == extensionInputTypeBoolean {
		switch strings.ToLower(answer) {
		case "y", yesFlagName, toolBoolTrue:
			raw = json.RawMessage("true")
		case "n", "no", "false":
			raw = json.RawMessage("false")
		default:
			return nil, false, "Answer y or n."
		}
	}
	normalized, err := extensionpkg.NormalizeInputJSON(input.Type, raw)
	if err != nil {
		return nil, false, "Invalid value for the declared type (maximum 8 KB)."
	}
	return normalized, true, ""
}

func extensionInputPromptText(input extensionPromptInput) string {
	return safeLoopPromptText(cmp.Or(strings.TrimSpace(input.Prompt), input.ID))
}

func extensionInputPromptLabel(input extensionPromptInput) string {
	label := extensionInputPromptText(input)
	switch {
	case input.Type == extensionInputTypeSecret && input.State == contract.ExtensionInputStateSet:
		label += " (secret, set — Enter keeps it)"
	case input.Type == extensionInputTypeSecret:
		label += " (secret)"
	case input.State == contract.ExtensionInputStateSet:
		label += " (set — Enter keeps it)"
	case input.State == contract.ExtensionInputStateDaemonEnv:
		label += " (daemon environment — Enter keeps it unbound)"
	}
	var text string
	switch {
	case input.State == contract.ExtensionInputStateSet:
	case input.Type == extensionInputTypeBoolean && string(input.Default) == "true":
		label += " [Y/n]"
	case input.Type == extensionInputTypeBoolean:
		label += " [y/N]"
	case len(input.Default) > 0 && json.Unmarshal(input.Default, &text) == nil:
		label += " [" + safeLoopPromptText(text) + "]"
	}
	return label + ": "
}

// read returns one answer. A hidden answer on a terminal file disables echo. The blocking terminal read
// cannot be interrupted, so cancellation (Ctrl-C) restores the terminal and abandons the read; the CLI
// exits right after and the reading goroutine ends with the process. A context already cancelled at
// entry never starts the read, so no late echo disable can follow the restore.
func (p *extensionInputPrompter) read(ctx context.Context, hidden bool) (string, error) {
	readLine := func() (string, error) {
		line, err := p.reader.ReadString('\n')
		if errors.Is(err, io.EOF) && line == "" {
			return "", errExtensionInputPromptAborted
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("cli: read extension input: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	restore := func() error { return nil }
	if file, ok := p.input.(*os.File); ok && hidden && p.readPassword != nil {
		fd := int(file.Fd())
		if state, err := term.GetState(fd); err == nil {
			restore = func() error { return term.Restore(fd, state) }
		}
		readLine = func() (string, error) {
			value, err := p.readPassword(fd)
			_, writeErr := fmt.Fprintln(p.output)
			if err != nil {
				return "", errors.Join(fmt.Errorf("cli: read hidden extension input: %w", err), writeErr)
			}
			if writeErr != nil {
				return "", fmt.Errorf("cli: write extension input prompt: %w", writeErr)
			}
			return strings.TrimRight(string(value), "\r\n"), nil
		}
	}
	abort := func() (string, error) {
		err := errExtensionInputPromptAborted
		if restoreErr := restore(); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("cli: restore terminal: %w", restoreErr))
		}
		if _, writeErr := fmt.Fprintln(p.output); writeErr != nil {
			err = errors.Join(err, fmt.Errorf("cli: write extension input prompt: %w", writeErr))
		}
		return "", err
	}
	if ctx.Err() != nil {
		return abort()
	}
	type answer struct {
		text string
		err  error
	}
	done := make(chan answer, 1)
	go func() {
		text, err := readLine()
		done <- answer{text: text, err: err}
	}()
	select {
	case result := <-done:
		return result.text, result.err
	case <-ctx.Done():
		return abort()
	}
}
