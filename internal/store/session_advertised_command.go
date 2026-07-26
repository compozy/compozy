package store

import (
	"errors"
	"strings"
)

// SessionAdvertisedCommand is one ACP command in the latest replacement set for a session.
type SessionAdvertisedCommand struct {
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	Input       *SessionAdvertisedCommandInput `json:"input,omitempty"`
}

// SessionAdvertisedCommandInput describes the optional unstructured command argument.
type SessionAdvertisedCommandInput struct {
	Hint string `json:"hint"`
}

// Validate enforces the durable advertised-command projection contract.
func (c SessionAdvertisedCommand) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("store: advertised command name is required")
	}
	return nil
}

// CloneSessionAdvertisedCommands returns an ownership-safe copy of a command replacement set.
func CloneSessionAdvertisedCommands(commands []SessionAdvertisedCommand) []SessionAdvertisedCommand {
	if len(commands) == 0 {
		return []SessionAdvertisedCommand{}
	}
	cloned := make([]SessionAdvertisedCommand, 0, len(commands))
	for _, command := range commands {
		copyCommand := command
		if command.Input != nil {
			input := *command.Input
			copyCommand.Input = &input
		}
		cloned = append(cloned, copyCommand)
	}
	return cloned
}

// SessionAdvertisedCommandsEqual compares two typed command values.
func SessionAdvertisedCommandsEqual(left SessionAdvertisedCommand, right SessionAdvertisedCommand) bool {
	if left.Name != right.Name || left.Description != right.Description {
		return false
	}
	if left.Input == nil || right.Input == nil {
		return left.Input == nil && right.Input == nil
	}
	return left.Input.Hint == right.Input.Hint
}
