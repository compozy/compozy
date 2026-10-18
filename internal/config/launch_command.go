package config

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	shellquote "github.com/kballard/go-shellquote"
)

// ErrCommandMissingExecutable reports a command made only of environment assignments.
var ErrCommandMissingExecutable = errors.New("command is missing an executable")

// LaunchCommand separates private leading environment assignments from the executable and arguments.
type LaunchCommand struct {
	Executable  string
	Args        []string
	Environment []string
}

// ParseLaunchCommand splits one shell-style launch command without executing a shell.
// Leading NAME=value tokens become private environment forwarded literally: there is no
// "~" or variable expansion. Errors are unprefixed ("parse command: …" or
// ErrCommandMissingExecutable) so callers attach their own path or subsystem prefix.
// providerauth.ParseCommand delegates here; this is the single launch-command grammar.
func ParseLaunchCommand(command string) (LaunchCommand, error) {
	argv, err := shellquote.Split(command)
	if err != nil {
		return LaunchCommand{}, fmt.Errorf("parse command: %w", err)
	}
	if len(argv) == 0 {
		return LaunchCommand{}, errors.New("command is empty")
	}

	parsed := LaunchCommand{}
	executableIndex := 0
	for executableIndex < len(argv) && isEnvironmentAssignment(argv[executableIndex]) {
		parsed.Environment = append(parsed.Environment, argv[executableIndex])
		executableIndex++
	}
	if executableIndex == len(argv) {
		return LaunchCommand{}, ErrCommandMissingExecutable
	}
	parsed.Executable = argv[executableIndex]
	parsed.Args = append([]string(nil), argv[executableIndex+1:]...)
	return parsed, nil
}

// CommandFingerprint correlates launch routes without exposing secret-bearing command
// text. Events, logs, transcript markers, and projections all use this one function.
func CommandFingerprint(command string) string {
	if strings.TrimSpace(command) == "" {
		return ""
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(command)))
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
