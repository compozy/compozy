package providerauth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/providerenv"
	shellquote "github.com/kballard/go-shellquote"
)

// ParsedCommand separates private leading environment assignments from the executable and arguments.
type ParsedCommand struct {
	Executable  string
	Args        []string
	Environment []string
}

// ParseCommand parses one shell-style provider auth command without executing a shell.
func ParseCommand(command string) (ParsedCommand, error) {
	if strings.TrimSpace(command) == "" {
		return ParsedCommand{}, errors.New("provider auth: command is required")
	}
	argv, err := shellquote.Split(command)
	if err != nil {
		return ParsedCommand{}, fmt.Errorf("provider auth: parse command: %w", err)
	}
	if len(argv) == 0 {
		return ParsedCommand{}, errors.New("provider auth: command is empty")
	}

	parsed := ParsedCommand{}
	executableIndex := 0
	for executableIndex < len(argv) && isEnvironmentAssignment(argv[executableIndex]) {
		parsed.Environment = append(parsed.Environment, argv[executableIndex])
		executableIndex++
	}
	if executableIndex == len(argv) {
		return ParsedCommand{}, errors.New("provider auth: command is missing an executable")
	}
	parsed.Executable = argv[executableIndex]
	parsed.Args = append([]string(nil), argv[executableIndex+1:]...)
	return parsed, nil
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

func isEnvironmentAssignment(value string) bool {
	name, _, ok := strings.Cut(value, "=")
	if !ok || name == "" || !isEnvironmentNameStart(name[0]) {
		return false
	}
	for index := 1; index < len(name); index++ {
		if !isEnvironmentNamePart(name[index]) {
			return false
		}
	}
	return true
}

func isEnvironmentNameStart(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func isEnvironmentNamePart(value byte) bool {
	return isEnvironmentNameStart(value) || value >= '0' && value <= '9'
}
