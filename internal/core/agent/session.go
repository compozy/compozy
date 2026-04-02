package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

// Session represents an active ACP session streaming updates.
type Session interface {
	// ID returns the ACP session identifier.
	ID() string
	// Updates returns the streamed session updates in arrival order.
	Updates() <-chan model.SessionUpdate
	// Done closes when the session has fully completed.
	Done() <-chan struct{}
	// Err returns the terminal session error, if any.
	Err() error
}

type sessionImpl struct {
	id      string
	updates chan model.SessionUpdate
	done    chan struct{}

	mu          sync.RWMutex
	err         error
	finished    bool
	updatesSeen int
}

func newSession(id string) *sessionImpl {
	return &sessionImpl{
		id:      id,
		updates: make(chan model.SessionUpdate, 128),
		done:    make(chan struct{}),
	}
}

func (s *sessionImpl) Updates() <-chan model.SessionUpdate {
	return s.updates
}

func (s *sessionImpl) ID() string {
	return s.id
}

func (s *sessionImpl) Done() <-chan struct{} {
	return s.done
}

func (s *sessionImpl) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

func (s *sessionImpl) publish(update model.SessionUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	if update.Status == "" {
		update.Status = model.StatusRunning
	}
	s.updatesSeen++
	select {
	case s.updates <- update:
	default:
	}
}

func (s *sessionImpl) finish(status model.SessionStatus, err error) {
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	s.finished = true
	s.err = err
	select {
	case s.updates <- model.SessionUpdate{Status: status}:
	default:
	}
	close(s.updates)
	close(s.done)
	s.mu.Unlock()
}

func (s *sessionImpl) waitForIdle(ctx context.Context, idleWindow time.Duration) {
	s.mu.RLock()
	lastSeen := s.updatesSeen
	s.mu.RUnlock()

	timer := time.NewTimer(idleWindow)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s.mu.RLock()
			currentSeen := s.updatesSeen
			s.mu.RUnlock()
			if currentSeen == lastSeen {
				return
			}
			lastSeen = currentSeen
			timer.Reset(idleWindow)
		}
	}
}

func convertACPUpdate(update acp.SessionUpdate) (model.SessionUpdate, error) {
	blocks, err := extractACPUpdateBlocks(update)
	if err != nil {
		return model.SessionUpdate{}, err
	}

	return model.SessionUpdate{
		Blocks: blocks,
		Status: model.StatusRunning,
	}, nil
}

func extractACPUpdateBlocks(update acp.SessionUpdate) ([]model.ContentBlock, error) {
	switch {
	case update.UserMessageChunk != nil:
		return convertACPContentBlock(update.UserMessageChunk.Content)
	case update.AgentMessageChunk != nil:
		return convertACPContentBlock(update.AgentMessageChunk.Content)
	case update.AgentThoughtChunk != nil:
		return convertACPContentBlock(update.AgentThoughtChunk.Content)
	case update.ToolCall != nil:
		return convertACPToolCallStart(update.ToolCall)
	case update.ToolCallUpdate != nil:
		return convertToolCallContent(
			string(update.ToolCallUpdate.ToolCallId),
			update.ToolCallUpdate.Content,
			update.ToolCallUpdate.RawOutput,
			update.ToolCallUpdate.Status != nil && *update.ToolCallUpdate.Status == acp.ToolCallStatusFailed,
		)
	case update.Plan != nil:
		return convertACPJSONTextBlock(update.Plan.Entries, "plan update")
	case update.AvailableCommandsUpdate != nil:
		return convertACPJSONTextBlock(update.AvailableCommandsUpdate.AvailableCommands, "available commands update")
	case update.CurrentModeUpdate != nil:
		block, err := model.NewContentBlock(model.TextBlock{Text: string(update.CurrentModeUpdate.CurrentModeId)})
		if err != nil {
			return nil, err
		}
		return []model.ContentBlock{block}, nil
	default:
		return nil, nil
	}
}

func convertACPToolCallStart(toolCall *acp.SessionUpdateToolCall) ([]model.ContentBlock, error) {
	toolUseBlock, err := model.NewContentBlock(model.ToolUseBlock{
		ID:    string(toolCall.ToolCallId),
		Name:  toolCall.Title,
		Input: marshalRawJSON(toolCall.RawInput),
	})
	if err != nil {
		return nil, err
	}

	blocks := []model.ContentBlock{toolUseBlock}
	converted, err := convertToolCallContent(
		string(toolCall.ToolCallId),
		toolCall.Content,
		toolCall.RawOutput,
		toolCall.Status == acp.ToolCallStatusFailed,
	)
	if err != nil {
		return nil, err
	}
	blocks = append(blocks, converted...)
	return blocks, nil
}

func convertACPJSONTextBlock(value any, label string) ([]model.ContentBlock, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", label, err)
	}
	block, err := model.NewContentBlock(model.TextBlock{Text: string(payload)})
	if err != nil {
		return nil, err
	}
	return []model.ContentBlock{block}, nil
}

func convertACPContentBlock(block acp.ContentBlock) ([]model.ContentBlock, error) {
	switch {
	case block.Text != nil:
		typed, err := model.NewContentBlock(model.TextBlock{Text: block.Text.Text})
		if err != nil {
			return nil, err
		}
		return []model.ContentBlock{typed}, nil
	case block.Image != nil:
		typed, err := model.NewContentBlock(model.ImageBlock{
			Data:     block.Image.Data,
			MimeType: block.Image.MimeType,
			URI:      block.Image.Uri,
		})
		if err != nil {
			return nil, err
		}
		return []model.ContentBlock{typed}, nil
	case block.Audio != nil, block.ResourceLink != nil, block.Resource != nil:
		payload, err := json.Marshal(block)
		if err != nil {
			return nil, fmt.Errorf("marshal ACP content block fallback: %w", err)
		}
		typed, err := model.NewContentBlock(model.TextBlock{Text: string(payload)})
		if err != nil {
			return nil, err
		}
		return []model.ContentBlock{typed}, nil
	default:
		return nil, nil
	}
}

func convertToolCallContent(
	toolUseID string,
	content []acp.ToolCallContent,
	rawOutput any,
	isError bool,
) ([]model.ContentBlock, error) {
	blocks := make([]model.ContentBlock, 0, len(content)+1)
	for _, item := range content {
		switch {
		case item.Content != nil:
			text, imageBlocks, err := convertToolContentBlock(toolUseID, item.Content.Content, isError)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, text...)
			blocks = append(blocks, imageBlocks...)
		case item.Diff != nil:
			diffText := renderDiffText(item.Diff.Path, item.Diff.NewText, item.Diff.OldText)
			block, err := model.NewContentBlock(model.DiffBlock{
				FilePath: item.Diff.Path,
				Diff:     diffText,
				OldText:  item.Diff.OldText,
				NewText:  item.Diff.NewText,
			})
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
		case item.Terminal != nil:
			block, err := model.NewContentBlock(model.TerminalOutputBlock{
				TerminalID: item.Terminal.TerminalId,
				Output:     stringifyValue(rawOutput),
			})
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
		}
	}

	if len(blocks) == 0 && rawOutput != nil {
		block, err := model.NewContentBlock(model.ToolResultBlock{
			ToolUseID: toolUseID,
			Content:   stringifyValue(rawOutput),
			IsError:   isError,
		})
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

func convertToolContentBlock(
	toolUseID string,
	block acp.ContentBlock,
	isError bool,
) ([]model.ContentBlock, []model.ContentBlock, error) {
	switch {
	case block.Text != nil:
		textBlock, err := model.NewContentBlock(model.ToolResultBlock{
			ToolUseID: toolUseID,
			Content:   block.Text.Text,
			IsError:   isError,
		})
		if err != nil {
			return nil, nil, err
		}
		return []model.ContentBlock{textBlock}, nil, nil
	case block.Image != nil:
		imageBlock, err := model.NewContentBlock(model.ImageBlock{
			Data:     block.Image.Data,
			MimeType: block.Image.MimeType,
			URI:      block.Image.Uri,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, []model.ContentBlock{imageBlock}, nil
	default:
		payload, err := json.Marshal(block)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal tool content fallback: %w", err)
		}
		textBlock, err := model.NewContentBlock(model.ToolResultBlock{
			ToolUseID: toolUseID,
			Content:   string(payload),
			IsError:   isError,
		})
		if err != nil {
			return nil, nil, err
		}
		return []model.ContentBlock{textBlock}, nil, nil
	}
}

func marshalRawJSON(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	if raw, ok := value.(json.RawMessage); ok {
		return append(json.RawMessage(nil), raw...)
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return payload
}

func stringifyValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(payload)
}

func renderDiffText(path string, newText string, oldText *string) string {
	if oldText == nil {
		return fmt.Sprintf("+++ %s\n%s", path, newText)
	}
	return fmt.Sprintf("--- %s\n%s\n+++ %s\n%s", path, *oldText, path, newText)
}
