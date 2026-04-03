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
	id           string
	workingDir   string
	allowedRoots []string
	updates      chan model.SessionUpdate
	done         chan struct{}

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

func newSessionWithAccess(id string, workingDir string, allowedRoots []string) *sessionImpl {
	session := newSession(id)
	session.workingDir = workingDir
	session.allowedRoots = append([]string(nil), allowedRoots...)
	return session
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
	if converted, ok, err := convertACPMessageUpdate(update); ok || err != nil {
		return converted, err
	}
	if converted, ok, err := convertACPToolLifecycleUpdate(update); ok || err != nil {
		return converted, err
	}
	if converted, ok := convertACPSessionStateUpdate(update); ok {
		return converted, nil
	}
	return model.SessionUpdate{Status: model.StatusRunning}, nil
}

func convertACPMessageUpdate(update acp.SessionUpdate) (model.SessionUpdate, bool, error) {
	switch {
	case update.UserMessageChunk != nil:
		return model.SessionUpdate{
			Kind:   model.UpdateKindUserMessageChunk,
			Status: model.StatusRunning,
		}, true, nil
	case update.AgentMessageChunk != nil:
		blocks, err := convertACPContentBlock(update.AgentMessageChunk.Content)
		if err != nil {
			return model.SessionUpdate{}, true, err
		}
		return model.SessionUpdate{
			Kind:   model.UpdateKindAgentMessageChunk,
			Blocks: blocks,
			Status: model.StatusRunning,
		}, true, nil
	case update.AgentThoughtChunk != nil:
		blocks, err := convertACPContentBlock(update.AgentThoughtChunk.Content)
		if err != nil {
			return model.SessionUpdate{}, true, err
		}
		return model.SessionUpdate{
			Kind:          model.UpdateKindAgentThoughtChunk,
			ThoughtBlocks: blocks,
			Status:        model.StatusRunning,
		}, true, nil
	default:
		return model.SessionUpdate{}, false, nil
	}
}

func convertACPToolLifecycleUpdate(update acp.SessionUpdate) (model.SessionUpdate, bool, error) {
	switch {
	case update.ToolCall != nil:
		blocks, err := convertACPToolCallStart(update.ToolCall)
		if err != nil {
			return model.SessionUpdate{}, true, err
		}
		state := convertACPToolCallState(update.ToolCall.Status)
		if state == model.ToolCallStateUnknown {
			state = model.ToolCallStatePending
		}
		return model.SessionUpdate{
			Kind:          model.UpdateKindToolCallStarted,
			ToolCallID:    string(update.ToolCall.ToolCallId),
			ToolCallState: state,
			Blocks:        blocks,
			Status:        model.StatusRunning,
		}, true, nil
	case update.ToolCallUpdate != nil:
		blocks, err := convertToolCallContent(
			string(update.ToolCallUpdate.ToolCallId),
			update.ToolCallUpdate.Content,
			update.ToolCallUpdate.RawOutput,
			update.ToolCallUpdate.Status != nil && *update.ToolCallUpdate.Status == acp.ToolCallStatusFailed,
		)
		if err != nil {
			return model.SessionUpdate{}, true, err
		}
		state := model.ToolCallStateUnknown
		if update.ToolCallUpdate.Status != nil {
			state = convertACPToolCallState(*update.ToolCallUpdate.Status)
		}
		return model.SessionUpdate{
			Kind:          model.UpdateKindToolCallUpdated,
			ToolCallID:    string(update.ToolCallUpdate.ToolCallId),
			ToolCallState: state,
			Blocks:        blocks,
			Status:        model.StatusRunning,
		}, true, nil
	default:
		return model.SessionUpdate{}, false, nil
	}
}

func convertACPSessionStateUpdate(update acp.SessionUpdate) (model.SessionUpdate, bool) {
	switch {
	case update.Plan != nil:
		return model.SessionUpdate{
			Kind:        model.UpdateKindPlanUpdated,
			PlanEntries: convertACPPlanEntries(update.Plan.Entries),
			Status:      model.StatusRunning,
		}, true
	case update.AvailableCommandsUpdate != nil:
		return model.SessionUpdate{
			Kind:              model.UpdateKindAvailableCommandsUpdated,
			AvailableCommands: convertACPAvailableCommands(update.AvailableCommandsUpdate.AvailableCommands),
			Status:            model.StatusRunning,
		}, true
	case update.CurrentModeUpdate != nil:
		return model.SessionUpdate{
			Kind:          model.UpdateKindCurrentModeUpdated,
			CurrentModeID: string(update.CurrentModeUpdate.CurrentModeId),
			Status:        model.StatusRunning,
		}, true
	default:
		return model.SessionUpdate{}, false
	}
}

func convertACPToolCallState(status acp.ToolCallStatus) model.ToolCallState {
	switch status {
	case acp.ToolCallStatusPending:
		return model.ToolCallStatePending
	case acp.ToolCallStatusInProgress:
		return model.ToolCallStateInProgress
	case acp.ToolCallStatusCompleted:
		return model.ToolCallStateCompleted
	case acp.ToolCallStatusFailed:
		return model.ToolCallStateFailed
	default:
		return model.ToolCallStateUnknown
	}
}

func convertACPPlanEntries(entries []acp.PlanEntry) []model.SessionPlanEntry {
	if len(entries) == 0 {
		return nil
	}

	converted := make([]model.SessionPlanEntry, 0, len(entries))
	for _, entry := range entries {
		converted = append(converted, model.SessionPlanEntry{
			Content:  entry.Content,
			Priority: string(entry.Priority),
			Status:   string(entry.Status),
		})
	}
	return converted
}

func convertACPAvailableCommands(commands []acp.AvailableCommand) []model.SessionAvailableCommand {
	if len(commands) == 0 {
		return nil
	}

	converted := make([]model.SessionAvailableCommand, 0, len(commands))
	for _, command := range commands {
		item := model.SessionAvailableCommand{
			Name:        command.Name,
			Description: command.Description,
		}
		if command.Input != nil && command.Input.UnstructuredCommandInput != nil {
			item.ArgumentHint = command.Input.UnstructuredCommandInput.Hint
		}
		converted = append(converted, item)
	}
	return converted
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
