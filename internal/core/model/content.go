package model

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// ContentBlockType identifies the serialized variant carried by a ContentBlock.
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

// SessionUpdateKind identifies the ACP notification variant carried by a SessionUpdate.
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
	Type  ContentBlockType `json:"type"`
	ID    string           `json:"id"`
	Name  string           `json:"name"`
	Input json.RawMessage  `json:"input,omitempty"`
}

// ToolResultBlock carries tool output when a richer block type is not available.
type ToolResultBlock struct {
	Type      ContentBlockType `json:"type"`
	ToolUseID string           `json:"toolUseId"`
	Content   string           `json:"content"`
	IsError   bool             `json:"isError,omitempty"`
}

// DiffBlock carries file modification details.
type DiffBlock struct {
	Type     ContentBlockType `json:"type"`
	FilePath string           `json:"filePath"`
	Diff     string           `json:"diff"`
	OldText  *string          `json:"oldText,omitempty"`
	NewText  string           `json:"newText,omitempty"`
}

// TerminalOutputBlock carries terminal execution details.
type TerminalOutputBlock struct {
	Type       ContentBlockType `json:"type"`
	Command    string           `json:"command,omitempty"`
	Output     string           `json:"output,omitempty"`
	ExitCode   int              `json:"exitCode"`
	TerminalID string           `json:"terminalId,omitempty"`
}

// ImageBlock carries inline image data.
type ImageBlock struct {
	Type     ContentBlockType `json:"type"`
	Data     string           `json:"data"`
	MimeType string           `json:"mimeType"`
	URI      *string          `json:"uri,omitempty"`
}

// SessionPlanEntry describes one ACP plan entry.
type SessionPlanEntry struct {
	Content  string `json:"content"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}

// SessionAvailableCommand describes one ACP slash-command style command.
type SessionAvailableCommand struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	ArgumentHint string `json:"argumentHint,omitempty"`
}

// SessionUpdate is the Compozy-owned view of one streamed ACP update.
type SessionUpdate struct {
	Kind              SessionUpdateKind         `json:"kind,omitempty"`
	ToolCallID        string                    `json:"toolCallId,omitempty"`
	ToolCallState     ToolCallState             `json:"toolCallState,omitempty"`
	Blocks            []ContentBlock            `json:"blocks,omitempty"`
	ThoughtBlocks     []ContentBlock            `json:"thoughtBlocks,omitempty"`
	PlanEntries       []SessionPlanEntry        `json:"planEntries,omitempty"`
	AvailableCommands []SessionAvailableCommand `json:"availableCommands,omitempty"`
	CurrentModeID     string                    `json:"currentModeId,omitempty"`
	Usage             Usage                     `json:"usage,omitempty"`
	Status            SessionStatus             `json:"status"`
}

// Usage tracks session token consumption.
type Usage struct {
	InputTokens  int `json:"inputTokens,omitempty"`
	OutputTokens int `json:"outputTokens,omitempty"`
	TotalTokens  int `json:"totalTokens,omitempty"`
	CacheReads   int `json:"cacheReads,omitempty"`
	CacheWrites  int `json:"cacheWrites,omitempty"`
}

// Add accumulates usage from another update into the receiver.
func (u *Usage) Add(other Usage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.TotalTokens += other.TotalTokens
	u.CacheReads += other.CacheReads
	u.CacheWrites += other.CacheWrites
}

// Total returns the derived total token count when TotalTokens is not populated.
func (u Usage) Total() int {
	if u.TotalTokens != 0 {
		return u.TotalTokens
	}
	return u.InputTokens + u.OutputTokens
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
	if _, err := decodeContentBlock(envelope.Type, data); err != nil {
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

func decodeContentBlock(blockType ContentBlockType, data []byte) (any, error) {
	switch blockType {
	case BlockText:
		return decodeTextBlock(data)
	case BlockToolUse:
		return decodeToolUseBlock(data)
	case BlockToolResult:
		return decodeToolResultBlock(data)
	case BlockDiff:
		return decodeDiffBlock(data)
	case BlockTerminalOutput:
		return decodeTerminalOutputBlock(data)
	case BlockImage:
		return decodeImageBlock(data)
	default:
		return nil, fmt.Errorf("decode content block: unsupported type %q", blockType)
	}
}

func decodeTextBlock(data []byte) (TextBlock, error) {
	var block TextBlock
	if err := json.Unmarshal(data, &block); err != nil {
		return TextBlock{}, fmt.Errorf("decode %s block: %w", BlockText, err)
	}
	block = ensureTextBlock(block)
	if block.Type != BlockText {
		return TextBlock{}, fmt.Errorf("decode %s block: unexpected type %q", BlockText, block.Type)
	}
	return block, nil
}

func decodeToolUseBlock(data []byte) (ToolUseBlock, error) {
	var block ToolUseBlock
	if err := json.Unmarshal(data, &block); err != nil {
		return ToolUseBlock{}, fmt.Errorf("decode %s block: %w", BlockToolUse, err)
	}
	block = ensureToolUseBlock(block)
	if block.Type != BlockToolUse {
		return ToolUseBlock{}, fmt.Errorf("decode %s block: unexpected type %q", BlockToolUse, block.Type)
	}
	return block, nil
}

func decodeToolResultBlock(data []byte) (ToolResultBlock, error) {
	var block ToolResultBlock
	if err := json.Unmarshal(data, &block); err != nil {
		return ToolResultBlock{}, fmt.Errorf("decode %s block: %w", BlockToolResult, err)
	}
	block = ensureToolResultBlock(block)
	if block.Type != BlockToolResult {
		return ToolResultBlock{}, fmt.Errorf("decode %s block: unexpected type %q", BlockToolResult, block.Type)
	}
	return block, nil
}

func decodeDiffBlock(data []byte) (DiffBlock, error) {
	var block DiffBlock
	if err := json.Unmarshal(data, &block); err != nil {
		return DiffBlock{}, fmt.Errorf("decode %s block: %w", BlockDiff, err)
	}
	block = ensureDiffBlock(block)
	if block.Type != BlockDiff {
		return DiffBlock{}, fmt.Errorf("decode %s block: unexpected type %q", BlockDiff, block.Type)
	}
	return block, nil
}

func decodeTerminalOutputBlock(data []byte) (TerminalOutputBlock, error) {
	var block TerminalOutputBlock
	if err := json.Unmarshal(data, &block); err != nil {
		return TerminalOutputBlock{}, fmt.Errorf("decode %s block: %w", BlockTerminalOutput, err)
	}
	block = ensureTerminalOutputBlock(block)
	if block.Type != BlockTerminalOutput {
		return TerminalOutputBlock{}, fmt.Errorf("decode %s block: unexpected type %q", BlockTerminalOutput, block.Type)
	}
	return block, nil
}

func decodeImageBlock(data []byte) (ImageBlock, error) {
	var block ImageBlock
	if err := json.Unmarshal(data, &block); err != nil {
		return ImageBlock{}, fmt.Errorf("decode %s block: %w", BlockImage, err)
	}
	block = ensureImageBlock(block)
	if block.Type != BlockImage {
		return ImageBlock{}, fmt.Errorf("decode %s block: unexpected type %q", BlockImage, block.Type)
	}
	return block, nil
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
