package providerauth

import (
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providerenv"
)

// ParsedCommand separates private leading environment assignments from the executable and arguments.
type ParsedCommand struct {
	Executable  string
	Args        []string
	Environment []string
}

// ParseCommand parses one shell-style provider auth command without executing a shell.
// The grammar is owned by compozyconfig.ParseLaunchCommand so configuration validation
// and launch-time parsing can never disagree.
func ParseCommand(command string) (ParsedCommand, error) {
	if strings.TrimSpace(command) == "" {
		return ParsedCommand{}, errors.New("provider auth: command is required")
	}
	parsed, err := compozyconfig.ParseLaunchCommand(command)
	if err != nil {
		return ParsedCommand{}, fmt.Errorf("provider auth: %w", err)
	}
	return ParsedCommand{
		Executable:  parsed.Executable,
		Args:        parsed.Args,
		Environment: parsed.Environment,
	}, nil
}

// ApplyEnvironment returns base with this command's leading assignments applied in order.
func (p ParsedCommand) ApplyEnvironment(base []string) []string {
	environment := append([]string(nil), base...)
	for _, assignment := range p.Environment {
		name, value, _ := strings.Cut(assignment, "=")
		environment = providerenv.SetEnvValue(environment, name, value)
	}
	return environment
}
