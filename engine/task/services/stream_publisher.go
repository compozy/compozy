package services

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/pubsub"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	streamChannelPattern = "stream:tokens:%s"
	maxSegmentRunes      = 200
	publishTimeout       = 2 * time.Second
)

// StreamPublisher publishes task output chunks to streaming subscribers.
type StreamPublisher interface {
	Publish(ctx context.Context, cfg *task.Config, state *task.State)
}

// TextStreamPublisher delivers plain-text task output over the configured pub/sub provider.
type TextStreamPublisher struct {
	provider pubsub.Provider
}

// NewTextStreamPublisher constructs a text stream publisher backed by the given provider.
func NewTextStreamPublisher(provider pubsub.Provider) *TextStreamPublisher {
	if provider == nil {
		return nil
	}
	return &TextStreamPublisher{provider: provider}
}

// Publish emits best-effort text chunks for tasks lacking structured output schemas.
func (p *TextStreamPublisher) Publish(ctx context.Context, cfg *task.Config, state *task.State) {
	if p == nil || p.provider == nil || cfg == nil || state == nil {
		return
	}
	if cfg.OutputSchema != nil || state.Output == nil || state.TaskExecID.IsZero() {
		return
	}
	text := extractResponseText(state.Output)
	if text == "" {
		return
	}
	channel := fmt.Sprintf(streamChannelPattern, state.TaskExecID.String())
	lines := splitLines(text)
	log := logger.FromContext(ctx)
	for _, line := range lines {
		for _, segment := range segmentLine(line, maxSegmentRunes) {
			sanitized := strings.TrimSpace(core.RedactString(segment))
			if sanitized == "" {
				continue
			}
			pubCtx, cancel := context.WithTimeout(ctx, publishTimeout)
			err := p.provider.Publish(pubCtx, channel, []byte(sanitized))
			cancel()
			if err != nil {
				log.Warn("Failed to publish stream chunk",
					"task_exec_id", state.TaskExecID.String(),
					"channel", channel,
					"error", core.RedactError(err),
				)
				return
			}
		}
	}
}

func extractResponseText(output *core.Output) string {
	if output == nil {
		return ""
	}
	if response := firstString((*output)["response"]); response != "" {
		return response
	}
	if value := firstString((*output)[core.OutputRootKey]); value != "" {
		return value
	}
	for _, v := range *output {
		if s := firstString(v); s != "" {
			return s
		}
	}
	return ""
}

func firstString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case []byte:
		return string(v)
	default:
		return ""
	}
}

func splitLines(text string) []string {
	if text == "" {
		return []string{""}
	}
	replaced := strings.ReplaceAll(text, "\r\n", "\n")
	replaced = strings.ReplaceAll(replaced, "\r", "\n")
	return strings.Split(replaced, "\n")
}

func segmentLine(line string, limit int) []string {
	if limit <= 0 || utf8.RuneCountInString(line) <= limit {
		return []string{line}
	}
	segments := make([]string, 0, utf8.RuneCountInString(line)/limit+1)
	var builder strings.Builder
	count := 0
	for _, r := range line {
		builder.WriteRune(r)
		count++
		if count >= limit {
			segments = append(segments, builder.String())
			builder.Reset()
			count = 0
		}
	}
	if builder.Len() > 0 {
		segments = append(segments, builder.String())
	}
	if len(segments) == 0 {
		segments = append(segments, line)
	}
	return segments
}
