package kinds

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/contentblock"
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
	switch blockType {
	case BlockText:
		_, err := decodeBlock(
			data,
			BlockText,
			func(block TextBlock) ContentBlockType { return block.Type },
			func(block *TextBlock, expected ContentBlockType) { block.Type = expected },
		)
		return err
	case BlockToolUse:
		_, err := decodeBlock(
			data,
			BlockToolUse,
			func(block ToolUseBlock) ContentBlockType { return block.Type },
			func(block *ToolUseBlock, expected ContentBlockType) { block.Type = expected },
		)
		return err
	case BlockToolResult:
		_, err := decodeBlock(
			data,
			BlockToolResult,
			func(block ToolResultBlock) ContentBlockType { return block.Type },
			func(block *ToolResultBlock, expected ContentBlockType) { block.Type = expected },
		)
		return err
	case BlockDiff:
		_, err := decodeBlock(
			data,
			BlockDiff,
			func(block DiffBlock) ContentBlockType { return block.Type },
			func(block *DiffBlock, expected ContentBlockType) { block.Type = expected },
		)
		return err
	case BlockTerminalOutput:
		_, err := decodeBlock(
			data,
			BlockTerminalOutput,
			func(block TerminalOutputBlock) ContentBlockType { return block.Type },
			func(block *TerminalOutputBlock, expected ContentBlockType) { block.Type = expected },
		)
		return err
	case BlockImage:
		_, err := decodeBlock(
			data,
			BlockImage,
			func(block ImageBlock) ContentBlockType { return block.Type },
			func(block *ImageBlock, expected ContentBlockType) { block.Type = expected },
		)
		return err
	default:
		return fmt.Errorf("decode content block: unsupported type %q", blockType)
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
