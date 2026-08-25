package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const terminalCLIActorID = "operator"

func writeTerminalOpenAttachBanner(output io.Writer, terminal TerminalRecord) error {
	_, err := fmt.Fprintf(
		output,
		"● %s opened in %s (%s) — attached. Detach: Ctrl-\\ Ctrl-\\\n\n",
		terminal.ID,
		terminal.Cwd,
		filepath.Base(terminal.Shell),
	)
	if err != nil {
		return fmt.Errorf("cli: write terminal open banner: %w", err)
	}
	return nil
}

func writeTerminalAttachBanner(output io.Writer, terminal TerminalRecord, control bool) error {
	controller := terminalControllerLabel(terminal.Controller)
	var message string
	if control {
		message = fmt.Sprintf("[control taken from %s — you type now]\n", controller)
	} else {
		message = fmt.Sprintf(
			"[watching %s — controller: %s. Take control: `compozy terminal attach %s --control`. Detach: Ctrl-\\ Ctrl-\\]\n",
			terminal.ID,
			controller,
			terminal.ID,
		)
	}
	if _, err := io.WriteString(output, message); err != nil {
		return fmt.Errorf("cli: write terminal attach banner: %w", err)
	}
	return nil
}

func terminalControllerLabel(controller *TerminalControllerRecord) string {
	if controller == nil {
		return "no one"
	}
	return strings.TrimSpace(controller.Kind + " " + controller.ID)
}

func terminalControllerNeedsTakeover(controller *TerminalControllerRecord) bool {
	return controller == nil ||
		controller.Kind != terminalControllerHumanKind ||
		controller.ID != terminalCLIActorID
}

func confirmTerminalTakeover(cmd *cobra.Command, controllerID string) (bool, error) {
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Take control from %s? [y/N] ", controllerID); err != nil {
		return false, fmt.Errorf("cli: write terminal takeover confirmation: %w", err)
	}
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && line == "" {
		return false, fmt.Errorf("cli: read terminal takeover confirmation: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func writeTerminalDetachNotice(
	ctx context.Context,
	output io.Writer,
	client TerminalClient,
	workspaceID, terminalID string,
) error {
	terminal, err := client.GetTerminal(ctx, workspaceID, terminalID)
	if err != nil {
		return fmt.Errorf("cli: read terminal state after attach: %w", err)
	}
	if terminal.State != terminalStateRunning {
		return nil
	}
	if _, err := io.WriteString(output, "\n[detached — terminal keeps running]\n"); err != nil {
		return fmt.Errorf("cli: write terminal detach notice: %w", err)
	}
	return nil
}
