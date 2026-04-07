package run

import (
	"fmt"
	"io"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

func writeRenderedLines(dst io.Writer, lines []string) error {
	if dst == nil || len(lines) == 0 {
		return nil
	}

	var builder strings.Builder
	for _, line := range lines {
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	_, err := io.WriteString(dst, builder.String())
	return err
}

func renderContentBlocks(blocks []model.ContentBlock) ([]string, []string) {
	var outLines []string
	var errLines []string
	for _, block := range blocks {
		renderedOut, renderedErr := renderContentBlock(block)
		outLines = append(outLines, renderedOut...)
		errLines = append(errLines, renderedErr...)
	}
	return outLines, errLines
}

func renderContentBlock(block model.ContentBlock) ([]string, []string) {
	switch block.Type {
	case model.BlockText:
		return renderTextBlock(block)
	case model.BlockToolUse:
		return renderToolUseBlock(block)
	case model.BlockToolResult:
		return renderToolResultBlock(block)
	case model.BlockDiff:
		return renderDiffBlock(block)
	case model.BlockTerminalOutput:
		return renderTerminalOutputBlock(block)
	case model.BlockImage:
		return renderImageBlock(block)
	default:
		return []string{strings.TrimSpace(string(block.Data))}, nil
	}
}

func renderTextBlock(block model.ContentBlock) ([]string, []string) {
	textBlock, err := block.AsText()
	if err != nil {
		return renderDecodeFailure(block, err), nil
	}
	return splitRenderedText(textBlock.Text), nil
}

func renderToolUseBlock(block model.ContentBlock) ([]string, []string) {
	toolUse, err := block.AsToolUse()
	if err != nil {
		return renderDecodeFailure(block, err), nil
	}

	line := fmt.Sprintf("[TOOL] %s (%s)", toolUseDisplayTitle(toolUse), toolUse.ID)
	outLines := []string{line}
	payload := toolUse.Input
	if len(payload) == 0 {
		payload = toolUse.RawInput
	}
	if len(payload) > 0 {
		outLines = append(outLines, splitRenderedText(string(payload))...)
	}
	return outLines, nil
}

func renderToolResultBlock(block model.ContentBlock) ([]string, []string) {
	toolResult, err := block.AsToolResult()
	if err != nil {
		return renderDecodeFailure(block, err), nil
	}

	lines := splitRenderedText(toolResult.Content)
	if len(lines) == 0 {
		lines = []string{fmt.Sprintf("[TOOL RESULT] %s", toolResult.ToolUseID)}
	}
	if toolResult.IsError {
		return nil, lines
	}
	return lines, nil
}

func renderDiffBlock(block model.ContentBlock) ([]string, []string) {
	diffBlock, err := block.AsDiff()
	if err != nil {
		return renderDecodeFailure(block, err), nil
	}
	return splitRenderedText(diffBlock.Diff), nil
}

func renderTerminalOutputBlock(block model.ContentBlock) ([]string, []string) {
	terminalBlock, err := block.AsTerminalOutput()
	if err != nil {
		return renderDecodeFailure(block, err), nil
	}

	lines := make([]string, 0, 4)
	if terminalBlock.Command != "" {
		lines = append(lines, "$ "+terminalBlock.Command)
	}
	lines = append(lines, splitRenderedText(terminalBlock.Output)...)
	if terminalBlock.ExitCode != 0 {
		lines = append(lines, fmt.Sprintf("[exit code: %d]", terminalBlock.ExitCode))
	}
	return lines, nil
}

func renderImageBlock(block model.ContentBlock) ([]string, []string) {
	imageBlock, err := block.AsImage()
	if err != nil {
		return renderDecodeFailure(block, err), nil
	}

	location := "inline"
	if imageBlock.URI != nil && *imageBlock.URI != "" {
		location = *imageBlock.URI
	}
	return []string{fmt.Sprintf("[IMAGE] %s %s", imageBlock.MimeType, location)}, nil
}

func renderDecodeFailure(block model.ContentBlock, err error) []string {
	payload := strings.TrimSpace(string(block.Data))
	if payload == "" {
		payload = fmt.Sprintf("type=%s", block.Type)
	}
	return []string{fmt.Sprintf("[decode %s block failed] %v", block.Type, err), payload}
}

func splitRenderedText(text string) []string {
	if text == "" {
		return nil
	}

	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.Split(normalized, "\n")
}
