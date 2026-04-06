package run

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

type statusCoder interface {
	StatusCode() int
}

func newRuntimeEvent(runID string, kind events.EventKind, payload any) (events.Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return events.Event{}, fmt.Errorf("marshal %s payload: %w", kind, err)
	}
	return events.Event{
		RunID:   runID,
		Kind:    kind,
		Payload: raw,
	}, nil
}

func usagePayload(index int, usage model.Usage) kinds.UsageUpdatedPayload {
	return kinds.UsageUpdatedPayload{
		Index: index,
		Usage: publicUsage(usage),
	}
}

func publicUsage(usage model.Usage) kinds.Usage {
	return kinds.Usage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  usage.TotalTokens,
		CacheReads:   usage.CacheReads,
		CacheWrites:  usage.CacheWrites,
	}
}

func publicSessionUpdate(update model.SessionUpdate) (kinds.SessionUpdate, error) {
	blocks, err := publicContentBlocks(update.Blocks)
	if err != nil {
		return kinds.SessionUpdate{}, err
	}
	thoughtBlocks, err := publicContentBlocks(update.ThoughtBlocks)
	if err != nil {
		return kinds.SessionUpdate{}, err
	}

	planEntries := make([]kinds.SessionPlanEntry, 0, len(update.PlanEntries))
	for _, entry := range update.PlanEntries {
		planEntries = append(planEntries, kinds.SessionPlanEntry{
			Content:  entry.Content,
			Priority: entry.Priority,
			Status:   entry.Status,
		})
	}

	commands := make([]kinds.SessionAvailableCommand, 0, len(update.AvailableCommands))
	for _, cmd := range update.AvailableCommands {
		commands = append(commands, kinds.SessionAvailableCommand{
			Name:         cmd.Name,
			Description:  cmd.Description,
			ArgumentHint: cmd.ArgumentHint,
		})
	}

	return kinds.SessionUpdate{
		Kind:              kinds.SessionUpdateKind(update.Kind),
		ToolCallID:        update.ToolCallID,
		ToolCallState:     kinds.ToolCallState(update.ToolCallState),
		Blocks:            blocks,
		ThoughtBlocks:     thoughtBlocks,
		PlanEntries:       planEntries,
		AvailableCommands: commands,
		CurrentModeID:     update.CurrentModeID,
		Usage:             publicUsage(update.Usage),
		Status:            kinds.SessionStatus(update.Status),
	}, nil
}

func publicContentBlocks(blocks []model.ContentBlock) ([]kinds.ContentBlock, error) {
	if len(blocks) == 0 {
		return nil, nil
	}

	converted := make([]kinds.ContentBlock, 0, len(blocks))
	for _, block := range blocks {
		item, err := publicContentBlock(block)
		if err != nil {
			return nil, err
		}
		converted = append(converted, item)
	}
	return converted, nil
}

func publicContentBlock(block model.ContentBlock) (kinds.ContentBlock, error) {
	switch block.Type {
	case model.BlockText:
		value, err := block.AsText()
		if err != nil {
			return kinds.ContentBlock{}, fmt.Errorf("decode text block: %w", err)
		}
		return kinds.NewContentBlock(kinds.TextBlock{
			Type: kinds.BlockText,
			Text: value.Text,
		})
	case model.BlockToolUse:
		value, err := block.AsToolUse()
		if err != nil {
			return kinds.ContentBlock{}, fmt.Errorf("decode tool use block: %w", err)
		}
		return kinds.NewContentBlock(kinds.ToolUseBlock{
			Type:     kinds.BlockToolUse,
			ID:       value.ID,
			Name:     value.Name,
			Title:    value.Title,
			ToolName: value.ToolName,
			Input:    copyJSON(value.Input),
			RawInput: copyJSON(value.RawInput),
		})
	case model.BlockToolResult:
		value, err := block.AsToolResult()
		if err != nil {
			return kinds.ContentBlock{}, fmt.Errorf("decode tool result block: %w", err)
		}
		return kinds.NewContentBlock(kinds.ToolResultBlock{
			Type:      kinds.BlockToolResult,
			ToolUseID: value.ToolUseID,
			Content:   value.Content,
			IsError:   value.IsError,
		})
	case model.BlockDiff:
		value, err := block.AsDiff()
		if err != nil {
			return kinds.ContentBlock{}, fmt.Errorf("decode diff block: %w", err)
		}
		return kinds.NewContentBlock(kinds.DiffBlock{
			Type:     kinds.BlockDiff,
			FilePath: value.FilePath,
			Diff:     value.Diff,
			OldText:  value.OldText,
			NewText:  value.NewText,
		})
	case model.BlockTerminalOutput:
		value, err := block.AsTerminalOutput()
		if err != nil {
			return kinds.ContentBlock{}, fmt.Errorf("decode terminal output block: %w", err)
		}
		return kinds.NewContentBlock(kinds.TerminalOutputBlock{
			Type:       kinds.BlockTerminalOutput,
			Command:    value.Command,
			Output:     value.Output,
			ExitCode:   value.ExitCode,
			TerminalID: value.TerminalID,
		})
	case model.BlockImage:
		value, err := block.AsImage()
		if err != nil {
			return kinds.ContentBlock{}, fmt.Errorf("decode image block: %w", err)
		}
		return kinds.NewContentBlock(kinds.ImageBlock{
			Type:     kinds.BlockImage,
			Data:     value.Data,
			MimeType: value.MimeType,
			URI:      value.URI,
		})
	default:
		return kinds.ContentBlock{}, fmt.Errorf("unsupported content block type %q", block.Type)
	}
}

func copyJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func providerStatusCode(err error) int {
	if err == nil {
		return 200
	}
	var coder statusCoder
	if errors.As(err, &coder) {
		return coder.StatusCode()
	}
	return 0
}

func issueIDFromPath(path string) string {
	base := filepath.Base(strings.TrimSpace(path))
	if base == "." || base == string(filepath.Separator) {
		return strings.TrimSpace(path)
	}
	return base
}
