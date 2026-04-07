package model

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/contentblock"
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
	Type     ContentBlockType `json:"type"`
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Title    string           `json:"title,omitempty"`
	ToolName string           `json:"toolName,omitempty"`
	Input    json.RawMessage  `json:"input,omitempty"`
	RawInput json.RawMessage  `json:"rawInput,omitempty"`
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
	if err := contentblock.ValidatePayload(block); err != nil {
		return ContentBlock{}, err
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
	return decodeBlock(
		b.Data,
		BlockText,
		func(block TextBlock) ContentBlockType { return block.Type },
		func(block *TextBlock, expected ContentBlockType) { block.Type = expected },
	)
}

// AsToolUse decodes the block as a ToolUseBlock.
func (b ContentBlock) AsToolUse() (ToolUseBlock, error) {
	return decodeBlock(
		b.Data,
		BlockToolUse,
		func(block ToolUseBlock) ContentBlockType { return block.Type },
		func(block *ToolUseBlock, expected ContentBlockType) { block.Type = expected },
	)
}

// AsToolResult decodes the block as a ToolResultBlock.
func (b ContentBlock) AsToolResult() (ToolResultBlock, error) {
	return decodeBlock(
		b.Data,
		BlockToolResult,
		func(block ToolResultBlock) ContentBlockType { return block.Type },
		func(block *ToolResultBlock, expected ContentBlockType) { block.Type = expected },
	)
}

// AsDiff decodes the block as a DiffBlock.
func (b ContentBlock) AsDiff() (DiffBlock, error) {
	return decodeBlock(
		b.Data,
		BlockDiff,
		func(block DiffBlock) ContentBlockType { return block.Type },
		func(block *DiffBlock, expected ContentBlockType) { block.Type = expected },
	)
}

// AsTerminalOutput decodes the block as a TerminalOutputBlock.
func (b ContentBlock) AsTerminalOutput() (TerminalOutputBlock, error) {
	return decodeBlock(
		b.Data,
		BlockTerminalOutput,
		func(block TerminalOutputBlock) ContentBlockType { return block.Type },
		func(block *TerminalOutputBlock, expected ContentBlockType) { block.Type = expected },
	)
}

// AsImage decodes the block as an ImageBlock.
func (b ContentBlock) AsImage() (ImageBlock, error) {
	return decodeBlock(
		b.Data,
		BlockImage,
		func(block ImageBlock) ContentBlockType { return block.Type },
		func(block *ImageBlock, expected ContentBlockType) { block.Type = expected },
	)
}

// MarshalJSON preserves the canonical JSON payload stored in Data.
func (b ContentBlock) MarshalJSON() ([]byte, error) {
	return contentblock.MarshalEnvelopeJSON(b.Type, b.Data)
}

// UnmarshalJSON validates the payload and stores its canonical JSON form.
func (b *ContentBlock) UnmarshalJSON(data []byte) error {
	envelope, err := contentblock.UnmarshalEnvelopeJSON(data, validateContentBlock)
	if err != nil {
		return err
	}
	b.Type = envelope.Type
	b.Data = envelope.Data
	return nil
}

func marshalContentBlock(block any) (ContentBlock, error) {
	envelope, err := contentblock.MarshalEnvelope[ContentBlockType](block)
	if err != nil {
		return ContentBlock{}, err
	}
	return ContentBlock{
		Type: envelope.Type,
		Data: envelope.Data,
	}, nil
}

func validateContentBlock(blockType ContentBlockType, data []byte) error {
	_, err := decodeContentBlock(blockType, data)
	return err
}

func decodeContentBlock(blockType ContentBlockType, data []byte) (any, error) {
	switch blockType {
	case BlockText:
		return decodeBlock(
			data,
			BlockText,
			func(block TextBlock) ContentBlockType { return block.Type },
			func(block *TextBlock, expected ContentBlockType) { block.Type = expected },
		)
	case BlockToolUse:
		return decodeBlock(
			data,
			BlockToolUse,
			func(block ToolUseBlock) ContentBlockType { return block.Type },
			func(block *ToolUseBlock, expected ContentBlockType) { block.Type = expected },
		)
	case BlockToolResult:
		return decodeBlock(
			data,
			BlockToolResult,
			func(block ToolResultBlock) ContentBlockType { return block.Type },
			func(block *ToolResultBlock, expected ContentBlockType) { block.Type = expected },
		)
	case BlockDiff:
		return decodeBlock(
			data,
			BlockDiff,
			func(block DiffBlock) ContentBlockType { return block.Type },
			func(block *DiffBlock, expected ContentBlockType) { block.Type = expected },
		)
	case BlockTerminalOutput:
		return decodeBlock(
			data,
			BlockTerminalOutput,
			func(block TerminalOutputBlock) ContentBlockType { return block.Type },
			func(block *TerminalOutputBlock, expected ContentBlockType) { block.Type = expected },
		)
	case BlockImage:
		return decodeBlock(
			data,
			BlockImage,
			func(block ImageBlock) ContentBlockType { return block.Type },
			func(block *ImageBlock, expected ContentBlockType) { block.Type = expected },
		)
	default:
		return nil, fmt.Errorf("decode content block: unsupported type %q", blockType)
	}
}

func decodeBlock[T any](
	data []byte,
	expected ContentBlockType,
	blockType func(T) ContentBlockType,
	normalize func(*T, ContentBlockType),
) (T, error) {
	return contentblock.DecodeBlock(data, expected, blockType, normalize)
}

type contentBlockNormalizer interface {
	normalizeContentBlock() any
}

func (b TextBlock) normalizeContentBlock() any {
	return normalizeBlock(b, func(block *TextBlock) { block.Type = BlockText })
}

func (b ToolUseBlock) normalizeContentBlock() any {
	return normalizeBlock(b, func(block *ToolUseBlock) { block.Type = BlockToolUse })
}

func (b ToolResultBlock) normalizeContentBlock() any {
	return normalizeBlock(b, func(block *ToolResultBlock) { block.Type = BlockToolResult })
}

func (b DiffBlock) normalizeContentBlock() any {
	return normalizeBlock(b, func(block *DiffBlock) { block.Type = BlockDiff })
}

func (b TerminalOutputBlock) normalizeContentBlock() any {
	return normalizeBlock(b, func(block *TerminalOutputBlock) { block.Type = BlockTerminalOutput })
}

func (b ImageBlock) normalizeContentBlock() any {
	return normalizeBlock(b, func(block *ImageBlock) { block.Type = BlockImage })
}

func normalizeBlock[T any](block T, normalize func(*T)) T {
	if normalize != nil {
		normalize(&block)
	}
	return block
}
