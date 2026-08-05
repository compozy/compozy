package task

import (
	"context"
	"fmt"
	"sort"
	"time"
)

func (s *inMemoryManagerStore) ReserveTerminalRunCommand(
	_ context.Context,
	command TerminalRunCommand,
) (TerminalRunCommand, bool, error) {
	if existing, ok := s.terminalCommands[command.RunID()]; ok {
		if existing.TaskID() != command.TaskID() || existing.WorkspaceID() != command.WorkspaceID() {
			return TerminalRunCommand{}, false, ErrInvalidStatusTransition
		}
		return existing, false, nil
	}
	run, ok := s.runs[command.RunID()]
	if !ok {
		return TerminalRunCommand{}, false, ErrTaskRunNotFound
	}
	if run.TaskID != command.TaskID() || run.WorkspaceID != command.WorkspaceID() ||
		!command.Fence().Matches(run) {
		return TerminalRunCommand{}, false, ErrInvalidStatusTransition
	}
	s.terminalCommands[command.RunID()] = command
	return command, true, nil
}

func (s *inMemoryManagerStore) AdvanceTerminalRunCommandPhase(
	_ context.Context,
	command TerminalRunCommand,
	next TerminalRunCommandPhase,
	updatedAt time.Time,
) (TerminalRunCommand, error) {
	existing, ok := s.terminalCommands[command.RunID()]
	if !ok || existing.CommandID() != command.CommandID() || existing.Phase() != command.Phase() ||
		existing.TaskID() != command.TaskID() || existing.WorkspaceID() != command.WorkspaceID() {
		return TerminalRunCommand{}, fmt.Errorf("%w: task run %q", ErrTerminalRunCommandInProgress, command.RunID())
	}
	advanced, err := existing.Advance(next, updatedAt)
	if err != nil {
		return TerminalRunCommand{}, err
	}
	s.terminalCommands[command.RunID()] = advanced
	return advanced, nil
}

func (s *inMemoryManagerStore) ReleaseTerminalRunCommand(
	_ context.Context,
	command TerminalRunCommand,
) error {
	existing, ok := s.terminalCommands[command.RunID()]
	run, runOK := s.runs[command.RunID()]
	if !ok || !runOK || existing.CommandID() != command.CommandID() ||
		existing.TaskID() != command.TaskID() || existing.WorkspaceID() != command.WorkspaceID() ||
		existing.Phase() != command.Phase() ||
		!existing.Fence().Matches(run) {
		return fmt.Errorf("%w: task run %q", ErrTerminalRunCommandInProgress, command.RunID())
	}
	delete(s.terminalCommands, command.RunID())
	return nil
}

func (s *inMemoryManagerStore) ListTerminalRunCommands(
	_ context.Context,
) ([]TerminalRunCommand, error) {
	commands := make([]TerminalRunCommand, 0, len(s.terminalCommands))
	for _, command := range s.terminalCommands {
		commands = append(commands, command)
	}
	sort.Slice(commands, func(i, j int) bool {
		if commands[i].admittedAt.Equal(commands[j].admittedAt) {
			return commands[i].RunID() < commands[j].RunID()
		}
		return commands[i].admittedAt.Before(commands[j].admittedAt)
	})
	return commands, nil
}

func (s *inMemoryManagerStore) ConsumeTerminalRunCommand(
	_ context.Context,
	command TerminalRunCommand,
	expectedPhase TerminalRunCommandPhase,
) (TerminalRunCommand, error) {
	existing, ok := s.terminalCommands[command.RunID()]
	run, runOK := s.runs[command.RunID()]
	if !ok || !runOK || existing.CommandID() != command.CommandID() ||
		existing.TaskID() != command.TaskID() || existing.WorkspaceID() != command.WorkspaceID() ||
		existing.Phase() != expectedPhase || !existing.Fence().Matches(run) {
		return TerminalRunCommand{}, fmt.Errorf("%w: task run %q", ErrTerminalRunCommandInProgress, command.RunID())
	}
	delete(s.terminalCommands, command.RunID())
	return existing, nil
}
