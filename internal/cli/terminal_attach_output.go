package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func terminalExitedAttachError(terminal contract.TerminalInfoPayload) error {
	detail := "cause unknown"
	if terminal.Exit != nil {
		switch {
		case terminal.Exit.Signal != nil && strings.TrimSpace(string(*terminal.Exit.Signal)) != "":
			detail = "signaled " + strings.TrimSpace(string(*terminal.Exit.Signal))
		case terminal.Exit.Code != nil:
			detail = fmt.Sprintf("exited %d", *terminal.Exit.Code)
		case strings.TrimSpace(string(terminal.Exit.Cause)) != "":
			detail = strings.TrimSpace(string(terminal.Exit.Cause))
		}
	}
	err := &terminalpkg.Error{
		Code:    terminalpkg.ErrorCodeExited,
		Message: fmt.Sprintf("%s already exited (%s)", terminal.ID, detail),
		Err:     terminalpkg.ErrExited,
	}
	return withCommandExitCode(apiStatusExitCode(http.StatusConflict), err)
}

func writeTerminalOpenAttachBanner(output io.Writer, terminal contract.TerminalInfoPayload) error {
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

func writeTerminalAttachBanner(output io.Writer, terminal contract.TerminalInfoPayload) error {
	message := fmt.Sprintf(
		"[attached to %s — shared input is active. Detach: Ctrl-\\ Ctrl-\\]\n",
		terminal.ID,
	)
	if _, err := io.WriteString(output, message); err != nil {
		return fmt.Errorf("cli: write terminal attach banner: %w", err)
	}
	return nil
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
