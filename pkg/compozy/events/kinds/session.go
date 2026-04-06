package kinds

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// ContentBlockType identifies the serialized content block variant.
type ContentBlockType string

const (
	// BlockText carries plain text or markdown content.
	BlockText ContentBlockType = "text"
	// BlockToolUse carries a tool invocation announcement.
	BlockToolUse ContentBlockType = "tool_use"
	// BlockToolResult carries tool output that is not represented by a richer block.
	BlockToolResult ContentBlockType = "tool_result"
	// BlockDiff carries file modification details.
	BlockDiff ContentBlockType = "diff"
	// BlockTerminalOutput carries terminal execution output.
	BlockTerminalOutput ContentBlockType = "terminal_output"
	// BlockImage carries inline image data.
	BlockImage ContentBlockType = "image"
)

// SessionStatus describes the lifecycle state of a streamed session update.
type SessionStatus string

const (
	// StatusRunning marks an in-flight session update.
	StatusRunning SessionStatus = "running"
	// StatusCompleted marks a completed session.
	StatusCompleted SessionStatus = "completed"
	// StatusFailed marks a failed or canceled session.
	StatusFailed SessionStatus = "failed"
)

// SessionUpdateKind identifies the semantic variant of a session update.
type SessionUpdateKind string

const (
	// UpdateKindUnknown marks an update with no additional semantic classification.
	UpdateKindUnknown SessionUpdateKind = ""
	// UpdateKindUserMessageChunk marks a streamed user message chunk.
	UpdateKindUserMessageChunk SessionUpdateKind = "user_message_chunk"
	// UpdateKindAgentMessageChunk marks a streamed agent message chunk.
	UpdateKindAgentMessageChunk SessionUpdateKind = "agent_message_chunk"
	// UpdateKindAgentThoughtChunk marks a streamed agent thought chunk.
	UpdateKindAgentThoughtChunk SessionUpdateKind = "agent_thought_chunk"
	// UpdateKindToolCallStarted marks the start of a tool call lifecycle.
	UpdateKindToolCallStarted SessionUpdateKind = "tool_call_started"
	// UpdateKindToolCallUpdated marks an update to an existing tool call lifecycle.
	UpdateKindToolCallUpdated SessionUpdateKind = "tool_call_updated"
	// UpdateKindPlanUpdated marks a plan update.
	UpdateKindPlanUpdated SessionUpdateKind = "plan_updated"
	// UpdateKindAvailableCommandsUpdated marks an available commands update.
	UpdateKindAvailableCommandsUpdated SessionUpdateKind = "available_commands_updated"
	// UpdateKindCurrentModeUpdated marks a current mode update.
	UpdateKindCurrentModeUpdated SessionUpdateKind = "current_mode_updated"
)

// ToolCallState describes the lifecycle state of a tool call entry.
type ToolCallState string

const (
	// ToolCallStateUnknown marks a tool call without an explicit lifecycle state.
	ToolCallStateUnknown ToolCallState = ""
	// ToolCallStatePending marks a pending tool call.
	ToolCallStatePending ToolCallState = "pending"
	// ToolCallStateInProgress marks an in-flight tool call.
	ToolCallStateInProgress ToolCallState = "in_progress"
	// ToolCallStateCompleted marks a completed tool call.
	ToolCallStateCompleted ToolCallState = "completed"
	// ToolCallStateFailed marks a failed tool call.
	ToolCallStateFailed ToolCallState = "failed"
	// ToolCallStateWaitingForConfirmation is reserved for future permission-aware UX.
	ToolCallStateWaitingForConfirmation ToolCallState = "waiting_for_confirmation"
)

// ContentBlock stores one typed content payload in its canonical JSON form.
type ContentBlock struct {
	Type ContentBlockType `json:"type"`
	Data json.RawMessage  `json:"-"`
}

// TextBlock carries plain text or markdown output.
type TextBlock struct {
	Type ContentBlockType `json:"type"`
	Text string           `json:"text"`
}

// ToolUseBlock describes the start of a tool invocation.
type ToolUseBlock struct {
	Type     ContentBlockType `json:"type"`
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Title    string           `json:"title,omitempty"`
	ToolName string           `json:"tool_name,omitempty"`
	Input    json.RawMessage  `json:"input,omitempty"`
	RawInput json.RawMessage  `json:"raw_input,omitempty"`
}

// ToolResultBlock carries tool output when a richer block type is not available.
type ToolResultBlock struct {
	Type      ContentBlockType `json:"type"`
	ToolUseID string           `json:"tool_use_id"`
	Content   string           `json:"content"`
	IsError   bool             `json:"is_error,omitempty"`
}

// DiffBlock carries file modification details.
type DiffBlock struct {
	Type     ContentBlockType `json:"type"`
	FilePath string           `json:"file_path"`
	Diff     string           `json:"diff"`
	OldText  *string          `json:"old_text,omitempty"`
	NewText  string           `json:"new_text,omitempty"`
}

// TerminalOutputBlock carries terminal execution details.
type TerminalOutputBlock struct {
	Type       ContentBlockType `json:"type"`
	Command    string           `json:"command,omitempty"`
	Output     string           `json:"output,omitempty"`
	ExitCode   int              `json:"exit_code"`
	TerminalID string           `json:"terminal_id,omitempty"`
}

// ImageBlock carries inline image data.
type ImageBlock struct {
	Type     ContentBlockType `json:"type"`
	Data     string           `json:"data"`
	MimeType string           `json:"mime_type"`
	URI      *string          `json:"uri,omitempty"`
}

// SessionPlanEntry describes one plan entry.
type SessionPlanEntry struct {
	Content  string `json:"content"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}

// SessionAvailableCommand describes one slash-command style action.
type SessionAvailableCommand struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	ArgumentHint string `json:"argument_hint,omitempty"`
}

// SessionUpdate is the public view of one streamed ACP update.
type SessionUpdate struct {
	Kind              SessionUpdateKind         `json:"kind,omitempty"`
	ToolCallID        string                    `json:"tool_call_id,omitempty"`
	ToolCallState     ToolCallState             `json:"tool_call_state,omitempty"`
	Blocks            []ContentBlock            `json:"blocks,omitempty"`
	ThoughtBlocks     []ContentBlock            `json:"thought_blocks,omitempty"`
	PlanEntries       []SessionPlanEntry        `json:"plan_entries,omitempty"`
	AvailableCommands []SessionAvailableCommand `json:"available_commands,omitempty"`
	CurrentModeID     string                    `json:"current_mode_id,omitempty"`
	Usage             Usage                     `json:"usage,omitempty"`
	Status            SessionStatus             `json:"status"`
}

// SessionStartedPayload describes a new attached session.
type SessionStartedPayload struct {
	Index          int    `json:"index"`
	ACPSessionID   string `json:"acp_session_id"`
	AgentSessionID string `json:"agent_session_id,omitempty"`
	Resumed        bool   `json:"resumed,omitempty"`
}

// SessionUpdatePayload carries one streamed session update.
type SessionUpdatePayload struct {
	Index  int           `json:"index"`
	Update SessionUpdate `json:"update"`
}

// SessionCompletedPayload describes a completed session.
type SessionCompletedPayload struct {
	Index int   `json:"index"`
	Usage Usage `json:"usage,omitempty"`
}

// SessionFailedPayload describes a failed session.
type SessionFailedPayload struct {
	Index int    `json:"index"`
	Error string `json:"error,omitempty"`
	Usage Usage  `json:"usage,omitempty"`
}

// NewContentBlock encodes a typed block into the generic ContentBlock envelope.
func NewContentBlock(block any) (ContentBlock, error) {
	if block == nil {
		return ContentBlock{}, fmt.Errorf("marshal content block: nil payload")
	}

	value := reflect.ValueOf(block)
	if value.Kind() == reflect.Ptr && value.IsNil() {
		return ContentBlock{}, fmt.Errorf("marshal content block: nil %T", block)
	}

	normalizer, ok := block.(contentBlockNormalizer)
	if !ok {
		return ContentBlock{}, fmt.Errorf("marshal content block: unsupported payload type %T", block)
	}
	return marshalContentBlock(normalizer.normalizeContentBlock())
}

// Decode unmarshals the envelope into its typed block payload.
func (b ContentBlock) Decode() (any, error) {
	switch b.Type {
	case BlockText:
		return b.AsText()
	case BlockToolUse:
		return b.AsToolUse()
	case BlockToolResult:
		return b.AsToolResult()
	case BlockDiff:
		return b.AsDiff()
	case BlockTerminalOutput:
		return b.AsTerminalOutput()
	case BlockImage:
		return b.AsImage()
	default:
		return nil, fmt.Errorf("decode content block: unsupported type %q", b.Type)
	}
}

// AsText decodes the block as a TextBlock.
func (b ContentBlock) AsText() (TextBlock, error) {
	return decodeTextBlock(b.Data)
}

// AsToolUse decodes the block as a ToolUseBlock.
func (b ContentBlock) AsToolUse() (ToolUseBlock, error) {
	return decodeToolUseBlock(b.Data)
}

// AsToolResult decodes the block as a ToolResultBlock.
func (b ContentBlock) AsToolResult() (ToolResultBlock, error) {
	return decodeToolResultBlock(b.Data)
}

// AsDiff decodes the block as a DiffBlock.
func (b ContentBlock) AsDiff() (DiffBlock, error) {
	return decodeDiffBlock(b.Data)
}

// AsTerminalOutput decodes the block as a TerminalOutputBlock.
func (b ContentBlock) AsTerminalOutput() (TerminalOutputBlock, error) {
	return decodeTerminalOutputBlock(b.Data)
}

// AsImage decodes the block as an ImageBlock.
func (b ContentBlock) AsImage() (ImageBlock, error) {
	return decodeImageBlock(b.Data)
}

// MarshalJSON preserves the canonical JSON payload stored in Data.
func (b ContentBlock) MarshalJSON() ([]byte, error) {
	if b.Type == "" {
		return nil, fmt.Errorf("marshal content block: missing type")
	}
	if len(b.Data) == 0 {
		return nil, fmt.Errorf("marshal %s block: missing data", b.Type)
	}
	return b.Data, nil
}

// UnmarshalJSON validates the payload and stores its canonical JSON form.
func (b *ContentBlock) UnmarshalJSON(data []byte) error {
	var envelope struct {
		Type ContentBlockType `json:"type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("decode content block envelope: %w", err)
	}
	if envelope.Type == "" {
		return fmt.Errorf("decode content block envelope: missing type")
	}
	if err := validateContentBlock(envelope.Type, data); err != nil {
		return err
	}

	b.Type = envelope.Type
	b.Data = append(b.Data[:0], data...)
	return nil
}

func marshalContentBlock(block any) (ContentBlock, error) {
	data, err := json.Marshal(block)
	if err != nil {
		return ContentBlock{}, fmt.Errorf("marshal content block payload: %w", err)
	}

	var envelope struct {
		Type ContentBlockType `json:"type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return ContentBlock{}, fmt.Errorf("decode marshaled content block envelope: %w", err)
	}
	if envelope.Type == "" {
		return ContentBlock{}, fmt.Errorf("marshal content block payload: missing type")
	}

	return ContentBlock{
		Type: envelope.Type,
		Data: data,
	}, nil
}

func validateContentBlock(blockType ContentBlockType, data []byte) error {
	switch blockType {
	case BlockText:
		_, err := decodeTextBlock(data)
		return err
	case BlockToolUse:
		_, err := decodeToolUseBlock(data)
		return err
	case BlockToolResult:
		_, err := decodeToolResultBlock(data)
		return err
	case BlockDiff:
		_, err := decodeDiffBlock(data)
		return err
	case BlockTerminalOutput:
		_, err := decodeTerminalOutputBlock(data)
		return err
	case BlockImage:
		_, err := decodeImageBlock(data)
		return err
	default:
		return fmt.Errorf("decode content block: unsupported type %q", blockType)
	}
}

func decodeTextBlock(data []byte) (TextBlock, error) {
	return decodeContentBlock(data, BlockText, func(block TextBlock) ContentBlockType {
		return block.Type
	}, ensureTextBlock)
}

func decodeToolUseBlock(data []byte) (ToolUseBlock, error) {
	return decodeContentBlock(data, BlockToolUse, func(block ToolUseBlock) ContentBlockType {
		return block.Type
	}, ensureToolUseBlock)
}

func decodeToolResultBlock(data []byte) (ToolResultBlock, error) {
	return decodeContentBlock(data, BlockToolResult, func(block ToolResultBlock) ContentBlockType {
		return block.Type
	}, ensureToolResultBlock)
}

func decodeDiffBlock(data []byte) (DiffBlock, error) {
	return decodeContentBlock(data, BlockDiff, func(block DiffBlock) ContentBlockType {
		return block.Type
	}, ensureDiffBlock)
}

func decodeTerminalOutputBlock(data []byte) (TerminalOutputBlock, error) {
	return decodeContentBlock(data, BlockTerminalOutput, func(block TerminalOutputBlock) ContentBlockType {
		return block.Type
	}, ensureTerminalOutputBlock)
}

func decodeImageBlock(data []byte) (ImageBlock, error) {
	return decodeContentBlock(data, BlockImage, func(block ImageBlock) ContentBlockType {
		return block.Type
	}, ensureImageBlock)
}

func decodeContentBlock[T any](
	data []byte,
	expected ContentBlockType,
	blockType func(T) ContentBlockType,
	ensure func(T) T,
) (T, error) {
	var block T
	if err := json.Unmarshal(data, &block); err != nil {
		var zero T
		return zero, fmt.Errorf("decode %s block: %w", expected, err)
	}
	if got := blockType(block); got != expected {
		var zero T
		return zero, fmt.Errorf("decode %s block: unexpected type %q", expected, got)
	}
	return ensure(block), nil
}

func ensureTextBlock(block TextBlock) TextBlock {
	block.Type = BlockText
	return block
}

func ensureToolUseBlock(block ToolUseBlock) ToolUseBlock {
	block.Type = BlockToolUse
	return block
}

func ensureToolResultBlock(block ToolResultBlock) ToolResultBlock {
	block.Type = BlockToolResult
	return block
}

func ensureDiffBlock(block DiffBlock) DiffBlock {
	block.Type = BlockDiff
	return block
}

func ensureTerminalOutputBlock(block TerminalOutputBlock) TerminalOutputBlock {
	block.Type = BlockTerminalOutput
	return block
}

func ensureImageBlock(block ImageBlock) ImageBlock {
	block.Type = BlockImage
	return block
}

type contentBlockNormalizer interface {
	normalizeContentBlock() any
}

func (b TextBlock) normalizeContentBlock() any {
	return ensureTextBlock(b)
}

func (b ToolUseBlock) normalizeContentBlock() any {
	return ensureToolUseBlock(b)
}

func (b ToolResultBlock) normalizeContentBlock() any {
	return ensureToolResultBlock(b)
}

func (b DiffBlock) normalizeContentBlock() any {
	return ensureDiffBlock(b)
}

func (b TerminalOutputBlock) normalizeContentBlock() any {
	return ensureTerminalOutputBlock(b)
}

func (b ImageBlock) normalizeContentBlock() any {
	return ensureImageBlock(b)
}
