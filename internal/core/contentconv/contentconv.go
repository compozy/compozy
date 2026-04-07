package contentconv

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func PublicUsage(usage model.Usage) kinds.Usage {
	return kinds.Usage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  usage.TotalTokens,
		CacheReads:   usage.CacheReads,
		CacheWrites:  usage.CacheWrites,
	}
}

func InternalUsage(usage kinds.Usage) model.Usage {
	return model.Usage{
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  usage.TotalTokens,
		CacheReads:   usage.CacheReads,
		CacheWrites:  usage.CacheWrites,
	}
}

func PublicSessionUpdate(update model.SessionUpdate) (kinds.SessionUpdate, error) {
	blocks, err := PublicContentBlocks(update.Blocks)
	if err != nil {
		return kinds.SessionUpdate{}, err
	}
	thoughtBlocks, err := PublicContentBlocks(update.ThoughtBlocks)
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
		Usage:             PublicUsage(update.Usage),
		Status:            kinds.SessionStatus(update.Status),
	}, nil
}

func InternalSessionUpdate(update kinds.SessionUpdate) (model.SessionUpdate, error) {
	blocks, err := InternalContentBlocks(update.Blocks)
	if err != nil {
		return model.SessionUpdate{}, err
	}
	thoughtBlocks, err := InternalContentBlocks(update.ThoughtBlocks)
	if err != nil {
		return model.SessionUpdate{}, err
	}

	planEntries := make([]model.SessionPlanEntry, 0, len(update.PlanEntries))
	for _, entry := range update.PlanEntries {
		planEntries = append(planEntries, model.SessionPlanEntry{
			Content:  entry.Content,
			Priority: entry.Priority,
			Status:   entry.Status,
		})
	}

	commands := make([]model.SessionAvailableCommand, 0, len(update.AvailableCommands))
	for _, cmd := range update.AvailableCommands {
		commands = append(commands, model.SessionAvailableCommand{
			Name:         cmd.Name,
			Description:  cmd.Description,
			ArgumentHint: cmd.ArgumentHint,
		})
	}

	return model.SessionUpdate{
		Kind:              model.SessionUpdateKind(update.Kind),
		ToolCallID:        update.ToolCallID,
		ToolCallState:     model.ToolCallState(update.ToolCallState),
		Blocks:            blocks,
		ThoughtBlocks:     thoughtBlocks,
		PlanEntries:       planEntries,
		AvailableCommands: commands,
		CurrentModeID:     update.CurrentModeID,
		Usage:             InternalUsage(update.Usage),
		Status:            model.SessionStatus(update.Status),
	}, nil
}

func PublicContentBlocks(blocks []model.ContentBlock) ([]kinds.ContentBlock, error) {
	if len(blocks) == 0 {
		return nil, nil
	}

	converted := make([]kinds.ContentBlock, 0, len(blocks))
	for _, block := range blocks {
		item, err := PublicContentBlock(block)
		if err != nil {
			return nil, err
		}
		converted = append(converted, item)
	}
	return converted, nil
}

func PublicContentBlock(block model.ContentBlock) (kinds.ContentBlock, error) {
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

func InternalContentBlocks(blocks []kinds.ContentBlock) ([]model.ContentBlock, error) {
	if len(blocks) == 0 {
		return nil, nil
	}

	converted := make([]model.ContentBlock, 0, len(blocks))
	for _, block := range blocks {
		item, err := InternalContentBlock(block)
		if err != nil {
			return nil, err
		}
		converted = append(converted, item)
	}
	return converted, nil
}

func InternalContentBlock(block kinds.ContentBlock) (model.ContentBlock, error) {
	value, err := block.Decode()
	if err != nil {
		return model.ContentBlock{}, err
	}

	switch item := value.(type) {
	case kinds.TextBlock:
		return model.NewContentBlock(model.TextBlock{
			Type: model.BlockText,
			Text: item.Text,
		})
	case kinds.ToolUseBlock:
		return model.NewContentBlock(model.ToolUseBlock{
			Type:     model.BlockToolUse,
			ID:       item.ID,
			Name:     item.Name,
			Title:    item.Title,
			ToolName: item.ToolName,
			Input:    copyJSON(item.Input),
			RawInput: copyJSON(item.RawInput),
		})
	case kinds.ToolResultBlock:
		return model.NewContentBlock(model.ToolResultBlock{
			Type:      model.BlockToolResult,
			ToolUseID: item.ToolUseID,
			Content:   item.Content,
			IsError:   item.IsError,
		})
	case kinds.DiffBlock:
		return model.NewContentBlock(model.DiffBlock{
			Type:     model.BlockDiff,
			FilePath: item.FilePath,
			Diff:     item.Diff,
			OldText:  item.OldText,
			NewText:  item.NewText,
		})
	case kinds.TerminalOutputBlock:
		return model.NewContentBlock(model.TerminalOutputBlock{
			Type:       model.BlockTerminalOutput,
			Command:    item.Command,
			Output:     item.Output,
			ExitCode:   item.ExitCode,
			TerminalID: item.TerminalID,
		})
	case kinds.ImageBlock:
		return model.NewContentBlock(model.ImageBlock{
			Type:     model.BlockImage,
			Data:     item.Data,
			MimeType: item.MimeType,
			URI:      item.URI,
		})
	default:
		return model.ContentBlock{}, fmt.Errorf("unsupported UI content block type %T", value)
	}
}

func copyJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}
