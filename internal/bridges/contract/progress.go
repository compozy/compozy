package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	redactpkg "github.com/compozy/agh/internal/redact"
)

const (
	defaultProgressPreviewLimit = 160
	defaultProgressEditInterval = 1500 * time.Millisecond
	bridgePlatformDiscord       = "discord"
	bridgePlatformSlack         = "slack"
	bridgePlatformTelegram      = "telegram"
	maxToolProgressErrorRunes   = 160
	// MaxToolProgressErrorRunes caps provider-facing progress error details.
	MaxToolProgressErrorRunes = maxToolProgressErrorRunes
)

// ToolProgressPhase is one closed tool lifecycle phase.
type ToolProgressPhase string

const (
	ToolProgressPhaseStarted   ToolProgressPhase = "started"
	ToolProgressPhaseCompleted ToolProgressPhase = "completed"
	ToolProgressPhaseFailed    ToolProgressPhase = "failed"
)

// ToolProgressPhaseValues returns the closed wire values in stable order.
func ToolProgressPhaseValues() []string {
	return []string{
		string(ToolProgressPhaseStarted),
		string(ToolProgressPhaseCompleted),
		string(ToolProgressPhaseFailed),
	}
}

// Normalize returns the canonical progress phase.
func (p ToolProgressPhase) Normalize() ToolProgressPhase {
	return ToolProgressPhase(strings.ToLower(strings.TrimSpace(string(p))))
}

// Validate reports whether the phase belongs to the progress contract.
func (p ToolProgressPhase) Validate() error {
	switch p.Normalize() {
	case ToolProgressPhaseStarted, ToolProgressPhaseCompleted, ToolProgressPhaseFailed:
		return nil
	case "":
		return errors.New("bridges: tool progress phase is required")
	default:
		return fmt.Errorf("bridges: unsupported tool progress phase %q", p)
	}
}

// ToolProgress is presentation-only tool lifecycle data.
type ToolProgress struct {
	ToolCallID string            `json:"tool_call_id"`
	ToolID     string            `json:"tool_id"`
	Phase      ToolProgressPhase `json:"phase"`
	Label      string            `json:"label"`
	Preview    string            `json:"preview,omitempty"`
	Emoji      string            `json:"emoji,omitempty"`
	DurationMS int64             `json:"duration_ms,omitempty"`
	Error      string            `json:"error,omitempty"`
	Index      int               `json:"index"`
}

// NormalizeToolProgress returns canonical presentation-only progress data.
func NormalizeToolProgress(progress ToolProgress) ToolProgress {
	return progress.normalize()
}

// Validate enforces the typed, redacted progress wire contract.
func (p ToolProgress) Validate() error {
	normalized := p.normalize()
	if err := requireField(normalized.ToolCallID, "tool progress tool call id"); err != nil {
		return err
	}
	if err := requireField(normalized.ToolID, "tool progress tool id"); err != nil {
		return err
	}
	if err := normalized.Phase.Validate(); err != nil {
		return err
	}
	if normalized.DurationMS < 0 {
		return errors.New("bridges: tool progress duration cannot be negative")
	}
	if normalized.Index <= 0 {
		return errors.New("bridges: tool progress index must be positive")
	}
	for label, value := range map[string]string{
		"label": normalized.Label, "preview": normalized.Preview,
		"emoji": normalized.Emoji, "error": normalized.Error,
	} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("bridges: tool progress %s must be one line", label)
		}
		if redactpkg.String(value) != value {
			return fmt.Errorf("bridges: tool progress %s contains secret-shaped material", label)
		}
	}
	if utf8.RuneCountInString(normalized.Error) > maxToolProgressErrorRunes {
		return fmt.Errorf("bridges: tool progress error exceeds %d runes", maxToolProgressErrorRunes)
	}
	return nil
}

func (p ToolProgress) normalize() ToolProgress {
	p.ToolCallID = strings.TrimSpace(p.ToolCallID)
	p.ToolID = strings.TrimSpace(p.ToolID)
	p.Phase = p.Phase.Normalize()
	p.Label = strings.TrimSpace(p.Label)
	p.Preview = strings.TrimSpace(p.Preview)
	p.Emoji = strings.TrimSpace(p.Emoji)
	p.Error = strings.TrimSpace(p.Error)
	return p
}

func cloneToolProgress(progress *ToolProgress) *ToolProgress {
	if progress == nil {
		return nil
	}
	cloned := progress.normalize()
	return &cloned
}

// ProgressMode controls which tool lifecycle events become presentation events.
type ProgressMode string

const (
	ProgressModeOff     ProgressMode = "off"
	ProgressModeNew     ProgressMode = "new"
	ProgressModeAll     ProgressMode = "all"
	ProgressModeVerbose ProgressMode = "verbose"
)

// Normalize returns the canonical progress mode.
func (m ProgressMode) Normalize() ProgressMode {
	return ProgressMode(strings.ToLower(strings.TrimSpace(string(m))))
}

// Validate reports whether the progress mode is supported.
func (m ProgressMode) Validate() error {
	switch m.Normalize() {
	case ProgressModeOff, ProgressModeNew, ProgressModeAll, ProgressModeVerbose:
		return nil
	case "":
		return errors.New("bridges: progress tool_progress is required")
	default:
		return fmt.Errorf("bridges: unsupported progress tool_progress %q", m)
	}
}

// ProgressGrouping controls whether progress lines accumulate or remain separate.
type ProgressGrouping string

const (
	ProgressGroupingAccumulate ProgressGrouping = "accumulate"
	ProgressGroupingSeparate   ProgressGrouping = "separate"
)

// Normalize returns the canonical progress grouping.
func (g ProgressGrouping) Normalize() ProgressGrouping {
	return ProgressGrouping(strings.ToLower(strings.TrimSpace(string(g))))
}

// Validate reports whether the progress grouping is supported.
func (g ProgressGrouping) Validate() error {
	switch g.Normalize() {
	case ProgressGroupingAccumulate, ProgressGroupingSeparate:
		return nil
	case "":
		return errors.New("bridges: progress grouping is required")
	default:
		return fmt.Errorf("bridges: unsupported progress grouping %q", g)
	}
}

// ProgressConfig is the effective bridge progress configuration.
type ProgressConfig struct {
	ToolProgress ProgressMode     `json:"tool_progress"`
	Grouping     ProgressGrouping `json:"grouping"`
	Typing       bool             `json:"typing"`
	Reactions    bool             `json:"reactions"`
	PreviewLimit int              `json:"-"`
	EditInterval time.Duration    `json:"-"`
}

// NormalizeProgressConfig applies the canonical internal defaults used by the
// wire contract.
func NormalizeProgressConfig(config ProgressConfig) ProgressConfig {
	return config.effective()
}

// Validate reports whether all progress settings are supported.
func (c ProgressConfig) Validate() error {
	if err := c.ToolProgress.Validate(); err != nil {
		return err
	}
	if err := c.Grouping.Validate(); err != nil {
		return err
	}
	if c.PreviewLimit < 0 {
		return errors.New("bridges: progress preview limit cannot be negative")
	}
	if c.EditInterval < 0 {
		return errors.New("bridges: progress edit interval cannot be negative")
	}
	return nil
}

func (c ProgressConfig) effective() ProgressConfig {
	c.ToolProgress = c.ToolProgress.Normalize()
	c.Grouping = c.Grouping.Normalize()
	if c.PreviewLimit == 0 && c.ToolProgress != ProgressModeVerbose {
		c.PreviewLimit = defaultProgressPreviewLimit
	}
	if c.EditInterval == 0 {
		c.EditInterval = defaultProgressEditInterval
	}
	return c
}

// ResolveProgressConfig applies instance, platform, and global defaults.
func ResolveProgressConfig(instance *BridgeInstance, platform string) ProgressConfig {
	if instance != nil {
		if override, ok := progressConfigOverride(instance.DeliveryDefaults); ok {
			return override.effective()
		}
		if strings.TrimSpace(platform) == "" {
			platform = instance.Platform
		}
	}
	config := ProgressConfig{
		ToolProgress: ProgressModeOff,
		Grouping:     ProgressGroupingAccumulate,
	}
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case bridgePlatformSlack, bridgePlatformTelegram, bridgePlatformDiscord:
		config.ToolProgress = ProgressModeNew
		config.Typing = true
		config.Reactions = true
	}
	return config.effective()
}

func progressConfigOverride(raw json.RawMessage) (ProgressConfig, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ProgressConfig{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ProgressConfig{}, false
	}
	progress, ok := fields["progress"]
	if !ok {
		return ProgressConfig{}, false
	}
	config, err := decodeProgressConfig(progress)
	if err != nil {
		return ProgressConfig{}, false
	}
	return config, true
}

// NormalizeDeliveryDefaultsJSON validates and canonicalizes delivery defaults.
func NormalizeDeliveryDefaultsJSON(raw json.RawMessage) (json.RawMessage, error) {
	normalized, err := normalizeRawJSON(raw, "bridge instance delivery defaults")
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 || bytes.Equal(normalized, []byte("null")) {
		return nil, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(normalized, &fields); err != nil {
		return nil, fmt.Errorf("bridges: bridge instance delivery defaults must be a JSON object or null: %w", err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := fields[key]
		if key == "progress" {
			config, decodeErr := decodeProgressConfig(value)
			if decodeErr != nil {
				return nil, decodeErr
			}
			canonical, marshalErr := json.Marshal(config)
			if marshalErr != nil {
				return nil, fmt.Errorf("bridges: marshal canonical delivery progress config: %w", marshalErr)
			}
			fields[key] = canonical
			continue
		}
		text, fieldErr := requireDeliveryDefaultStringField(value, key)
		if fieldErr != nil {
			return nil, fieldErr
		}
		if key != "mode" {
			continue
		}
		mode := DeliveryMode(text).Normalize()
		if err := mode.Validate(); err != nil {
			return nil, err
		}
		canonical, marshalErr := json.Marshal(mode)
		if marshalErr != nil {
			return nil, fmt.Errorf("bridges: marshal canonical delivery mode: %w", marshalErr)
		}
		fields[key] = canonical
	}
	canonical, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("bridges: marshal canonical bridge instance delivery defaults: %w", err)
	}
	return canonical, nil
}

func decodeProgressConfig(raw json.RawMessage) (ProgressConfig, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var config ProgressConfig
	if err := decoder.Decode(&config); err != nil {
		return ProgressConfig{}, fmt.Errorf("bridges: invalid delivery progress config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ProgressConfig{}, errors.New("bridges: invalid trailing data in delivery progress config")
	}
	if err := config.Validate(); err != nil {
		return ProgressConfig{}, err
	}
	return config.effective(), nil
}

func requireDeliveryDefaultStringField(raw json.RawMessage, field string) (string, error) {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf(
			"bridges: bridge instance delivery defaults field %q must be valid JSON: %w",
			field,
			err,
		)
	}
	text, ok := decoded.(string)
	if !ok {
		return "", fmt.Errorf("bridges: bridge instance delivery defaults field %q must be a string", field)
	}
	return text, nil
}
