package bridgesdk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

const (
	defaultProgressEditInterval  = 1500 * time.Millisecond
	defaultProgressMaxMessageLen = 4_000
	progressMessageHeadroom      = 64
)

// ProgressSink applies provider-specific progress messages and affordances.
type ProgressSink interface {
	Post(ctx context.Context, target bridgepkg.DeliveryTarget, text string) (remoteID string, err error)
	Edit(ctx context.Context, target bridgepkg.DeliveryTarget, remoteID string, text string) error
	Typing(ctx context.Context, target bridgepkg.DeliveryTarget, active bool) error
	React(ctx context.Context, target bridgepkg.DeliveryTarget, reaction string) error
}

// ProgressConfig binds settings to one target; empty mode/grouping default to all/accumulate.
type ProgressConfig struct {
	bridgepkg.ProgressConfig
	Target        bridgepkg.DeliveryTarget
	MaxMessageLen int              // Values through 64 use the conservative 4,000-unit default.
	Len           func(string) int // Nil uses byte length.
}

// ProgressAccumulator owns one route's mutable progress bubble state.
type ProgressAccumulator struct {
	mu sync.Mutex

	config    ProgressConfig
	configErr error
	sink      ProgressSink
	now       func() time.Time

	currentBubbleID   string
	currentText       string
	lastLine          string
	repeatCount       int
	lastEditAt        time.Time
	canEdit           bool
	dirty             bool
	typingActive      bool
	lastReactionPhase bridgepkg.ToolProgressPhase
}

// NewProgressAccumulator constructs state; a zero edit interval defaults to 1.5 seconds.
func NewProgressAccumulator(
	cfg ProgressConfig,
	sink ProgressSink,
	now func() time.Time,
) *ProgressAccumulator {
	normalized, err := normalizeProgressAccumulatorConfig(cfg)
	if sink == nil {
		sink = noOpProgressSink{}
	}
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &ProgressAccumulator{
		config:    normalized,
		configErr: err,
		sink:      sink,
		now:       now,
		canEdit:   normalized.Grouping == bridgepkg.ProgressGroupingAccumulate,
	}
}

// OnProgress records one tool lifecycle line and applies due provider updates.
func (a *ProgressAccumulator) OnProgress(ctx context.Context, event bridgepkg.ToolProgress) error {
	if a == nil {
		return errors.New("bridgesdk: progress accumulator is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.configErr != nil {
		return a.configErr
	}
	if a.config.ToolProgress == bridgepkg.ProgressModeOff {
		return nil
	}
	if err := validateProgressContext(ctx); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("bridgesdk: validate progress event: %w", err)
	}
	typingErr := a.applyTypingAffordanceLocked(ctx, event.Phase)
	line := renderProgressLine(event)
	var progressErr error
	switch {
	case a.config.Grouping == bridgepkg.ProgressGroupingSeparate:
		progressErr = a.postSeparateLineLocked(ctx, line)
	case line == a.lastLine && a.repeatCount > 0:
		progressErr = a.applyRepeatLocked(ctx)
	default:
		var appended bool
		appended, progressErr = a.appendLineLocked(ctx, line)
		if appended {
			a.lastLine = line
			a.repeatCount = 1
		}
	}
	if progressErr != nil {
		return errors.Join(progressErr, typingErr)
	}
	reactionErr := a.applyReactionAffordanceLocked(ctx, event.Phase)
	return errors.Join(typingErr, reactionErr)
}

// OnContent flushes pending progress, stops typing, and closes the current editable bubble.
func (a *ProgressAccumulator) OnContent(ctx context.Context) error {
	if a == nil {
		return errors.New("bridgesdk: progress accumulator is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.configErr != nil {
		return a.configErr
	}
	if a.config.ToolProgress == bridgepkg.ProgressModeOff {
		a.resetBubbleLocked()
		return nil
	}
	if err := validateProgressContext(ctx); err != nil {
		return err
	}
	flushErr := a.flushLocked(ctx, true)
	typingErr := a.closeTypingLocked(ctx)
	if flushErr == nil {
		a.resetBubbleLocked()
	}
	return errors.Join(flushErr, typingErr)
}

// Flush writes pending accumulated progress immediately.
func (a *ProgressAccumulator) Flush(ctx context.Context) error {
	if a == nil {
		return errors.New("bridgesdk: progress accumulator is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.configErr != nil {
		return a.configErr
	}
	if a.config.ToolProgress == bridgepkg.ProgressModeOff {
		return nil
	}
	if err := validateProgressContext(ctx); err != nil {
		return err
	}
	return a.flushLocked(ctx, true)
}

func (a *ProgressAccumulator) flushDue(ctx context.Context) error {
	if a == nil {
		return errors.New("bridgesdk: progress accumulator is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.configErr != nil {
		return a.configErr
	}
	if a.config.ToolProgress == bridgepkg.ProgressModeOff {
		return nil
	}
	if err := validateProgressContext(ctx); err != nil {
		return err
	}
	return a.flushLocked(ctx, false)
}

func normalizeProgressAccumulatorConfig(cfg ProgressConfig) (ProgressConfig, error) {
	cfg.ToolProgress = cfg.ToolProgress.Normalize()
	if cfg.ToolProgress == "" {
		cfg.ToolProgress = bridgepkg.ProgressModeAll
	}
	cfg.Grouping = cfg.Grouping.Normalize()
	if cfg.Grouping == "" {
		cfg.Grouping = bridgepkg.ProgressGroupingAccumulate
	}
	if cfg.EditInterval == 0 {
		cfg.EditInterval = defaultProgressEditInterval
	}
	if cfg.MaxMessageLen <= progressMessageHeadroom {
		cfg.MaxMessageLen = defaultProgressMaxMessageLen
	}
	if cfg.Len == nil {
		cfg.Len = func(value string) int {
			return len(value)
		}
	}
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("bridgesdk: invalid progress config: %w", err)
	}
	if !cfg.Target.IsZero() {
		if err := cfg.Target.Validate(); err != nil {
			return cfg, fmt.Errorf("bridgesdk: invalid progress target: %w", err)
		}
	}
	return cfg, nil
}

func validateProgressContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("bridgesdk: progress context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("bridgesdk: progress context: %w", err)
	}
	return nil
}

// PendingFlushDelay reports when a throttled accumulated edit is due.
func (a *ProgressAccumulator) PendingFlushDelay() (time.Duration, bool) {
	if a == nil {
		return 0, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.configErr != nil ||
		a.config.ToolProgress == bridgepkg.ProgressModeOff ||
		!a.dirty ||
		a.currentBubbleID == "" {
		return 0, false
	}
	delay := a.config.EditInterval - a.now().Sub(a.lastEditAt)
	delay = max(delay, 0)
	return delay, true
}

func (a *ProgressAccumulator) applyTypingAffordanceLocked(
	ctx context.Context,
	phase bridgepkg.ToolProgressPhase,
) error {
	active := phase == bridgepkg.ToolProgressPhaseStarted
	if !a.config.Typing {
		return nil
	}
	if err := a.sink.Typing(ctx, a.config.Target, active); err != nil {
		return fmt.Errorf("bridgesdk: set progress typing state: %w", err)
	}
	a.typingActive = active
	return nil
}

func (a *ProgressAccumulator) applyReactionAffordanceLocked(
	ctx context.Context,
	phase bridgepkg.ToolProgressPhase,
) error {
	if !a.config.Reactions || phase == a.lastReactionPhase {
		return nil
	}
	reaction := "👀"
	switch phase {
	case bridgepkg.ToolProgressPhaseCompleted:
		reaction = "✅"
	case bridgepkg.ToolProgressPhaseFailed:
		reaction = "❌"
	}
	if err := a.sink.React(ctx, a.config.Target, reaction); err != nil {
		if IsCommittedMutation(err) {
			a.lastReactionPhase = phase
		}
		return fmt.Errorf("bridgesdk: apply progress reaction: %w", err)
	}
	a.lastReactionPhase = phase
	return nil
}

func (a *ProgressAccumulator) closeTypingLocked(ctx context.Context) error {
	if !a.config.Typing || !a.typingActive {
		return nil
	}
	if err := a.sink.Typing(ctx, a.config.Target, false); err != nil {
		return fmt.Errorf("bridgesdk: set progress typing state: %w", err)
	}
	a.typingActive = false
	return nil
}

func (a *ProgressAccumulator) postSeparateLineLocked(ctx context.Context, line string) error {
	chunks, err := splitProgressText(line, a.messageLimitLocked(), a.config.Len, "")
	if err != nil {
		return err
	}
	for _, chunk := range chunks {
		if _, err := a.sink.Post(ctx, a.config.Target, chunk); err != nil {
			return fmt.Errorf("bridgesdk: post separate progress line: %w", err)
		}
	}
	return nil
}

func (a *ProgressAccumulator) flushLocked(ctx context.Context, force bool) error {
	if !a.dirty || a.currentText == "" {
		return nil
	}
	if got, limit := a.config.Len(a.currentText), a.messageLimitLocked(); got > limit {
		return fmt.Errorf("bridgesdk: progress message length %d exceeds provider limit %d", got, limit)
	}
	if a.currentBubbleID == "" || !a.canEdit {
		remoteID, err := a.sink.Post(ctx, a.config.Target, a.currentText)
		if err != nil {
			if IsCommittedMutation(err) {
				a.resetBubbleLocked()
			}
			return fmt.Errorf("bridgesdk: post progress bubble: %w", err)
		}
		a.currentBubbleID = strings.TrimSpace(remoteID)
		if a.currentBubbleID == "" {
			a.resetBubbleLocked()
			return &CommittedMutationError{
				Err: errors.New("bridgesdk: post progress bubble returned an empty remote id"),
			}
		}
		a.canEdit = true
		a.lastEditAt = a.now()
		a.dirty = false
		return nil
	}

	now := a.now()
	if !force && now.Sub(a.lastEditAt) < a.config.EditInterval {
		return nil
	}
	if err := a.sink.Edit(ctx, a.config.Target, a.currentBubbleID, a.currentText); err != nil {
		if IsCommittedMutation(err) {
			a.lastEditAt = now
			a.dirty = false
		}
		return fmt.Errorf("bridgesdk: edit progress bubble: %w", err)
	}
	a.lastEditAt = now
	a.dirty = false
	return nil
}

func (a *ProgressAccumulator) messageLimitLocked() int {
	return a.config.MaxMessageLen - progressMessageHeadroom
}

func (a *ProgressAccumulator) freezeCurrentBubbleLocked() {
	a.currentBubbleID = ""
	a.currentText = ""
	a.lastEditAt = time.Time{}
	a.canEdit = a.config.Grouping == bridgepkg.ProgressGroupingAccumulate
	a.dirty = false
}

func (a *ProgressAccumulator) resetBubbleLocked() {
	a.freezeCurrentBubbleLocked()
	a.lastLine = ""
	a.repeatCount = 0
	a.lastReactionPhase = ""
}

func renderProgressLine(event bridgepkg.ToolProgress) string {
	label := strings.TrimSpace(event.Label)
	if label == "" {
		label = strings.TrimSpace(event.ToolID)
	}
	line := label
	if preview := strings.TrimSpace(event.Preview); preview != "" {
		line += " " + preview
	}
	emoji := strings.TrimSpace(event.Emoji)
	switch event.Phase {
	case bridgepkg.ToolProgressPhaseCompleted:
		emoji = "✅"
		if duration := time.Duration(event.DurationMS) * time.Millisecond; duration >= 30*time.Second {
			line += " · " + duration.Round(100*time.Millisecond).String()
		}
	case bridgepkg.ToolProgressPhaseFailed:
		emoji = "❌"
		if detail := strings.TrimSpace(event.Error); detail != "" {
			line += " — " + detail
		}
	}
	if emoji != "" {
		line = emoji + " " + line
	}
	return strings.TrimSpace(line)
}

func progressRepeatSuffix(count int) string {
	if count <= 1 {
		return ""
	}
	return fmt.Sprintf(" (×%d)", count)
}

type noOpProgressSink struct{}

var _ ProgressSink = noOpProgressSink{}

func (noOpProgressSink) Post(
	context.Context,
	bridgepkg.DeliveryTarget,
	string,
) (string, error) {
	return "noop", nil
}

func (noOpProgressSink) Edit(
	context.Context,
	bridgepkg.DeliveryTarget,
	string,
	string,
) error {
	return nil
}

func (noOpProgressSink) Typing(context.Context, bridgepkg.DeliveryTarget, bool) error {
	return nil
}

func (noOpProgressSink) React(context.Context, bridgepkg.DeliveryTarget, string) error {
	return nil
}
