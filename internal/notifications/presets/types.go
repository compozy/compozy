package presets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	eventspkg "github.com/compozy/agh/internal/events"
)

const (
	BuiltInDefaultVersion = "1"

	BuiltInTaskTerminal     = "task_terminal"
	BuiltInSessionUnhealthy = "session_unhealthy"
	BuiltInProviderFailure  = "provider_failure"
	BuiltInTaskRunPattern   = "task.run_*"

	CursorConsumerPrefix = "preset:"
)

var (
	ErrPresetNotFound      = errors.New("notifications: preset not found")
	ErrInvalidPreset       = errors.New("notifications: invalid preset")
	ErrPresetBuiltIn       = errors.New("notifications: built-in preset cannot be deleted")
	ErrPresetDuplicateName = errors.New("notifications: preset name already exists")
)

// Target is one bridge destination attached to a notification preset.
type Target struct {
	BridgeID       string                 `json:"bridge_id"`
	CanonicalRoute string                 `json:"canonical_route,omitempty"`
	DisplayName    string                 `json:"display_name,omitempty"`
	DeliveryMode   bridgepkg.DeliveryMode `json:"delivery_mode,omitempty"`
}

// Preset is the SQLite-authoritative notification fanout policy.
type Preset struct {
	Name                   string    `json:"name"`
	Events                 []string  `json:"events"`
	Targets                []Target  `json:"targets"`
	Filter                 string    `json:"filter,omitempty"`
	Enabled                bool      `json:"enabled"`
	BuiltIn                bool      `json:"built_in"`
	DefaultVersion         string    `json:"default_version,omitempty"`
	DefaultHash            string    `json:"default_hash,omitempty"`
	UserModified           bool      `json:"user_modified"`
	DefaultUpdateAvailable bool      `json:"default_update_available"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// Query filters preset list operations.
type Query struct {
	Enabled *bool  `json:"enabled,omitempty"`
	BuiltIn *bool  `json:"built_in,omitempty"`
	Name    string `json:"name,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// CreateRequest captures one operator-created preset.
type CreateRequest struct {
	Name    string   `json:"name"`
	Events  []string `json:"events"`
	Targets []Target `json:"targets,omitempty"`
	Filter  string   `json:"filter,omitempty"`
	Enabled bool     `json:"enabled,omitempty"`
}

// UpdateRequest captures mutable preset fields.
type UpdateRequest struct {
	Events  *[]string `json:"events,omitempty"`
	Targets *[]Target `json:"targets,omitempty"`
	Filter  *string   `json:"filter,omitempty"`
	Enabled *bool     `json:"enabled,omitempty"`
	Now     time.Time `json:"-"`
}

// Event is the normalized runtime event shape consumed by preset dispatch.
type Event struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	WorkspaceID string            `json:"workspace_id,omitempty"`
	AgentName   string            `json:"agent,omitempty"`
	Provider    string            `json:"provider,omitempty"`
	TaskID      string            `json:"task_id,omitempty"`
	RunID       string            `json:"run_id,omitempty"`
	Summary     string            `json:"summary,omitempty"`
	Outcome     eventspkg.Outcome `json:"outcome"`
	Sequence    int64             `json:"sequence"`
	Payload     json.RawMessage   `json:"payload,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
}

// DispatchResult summarizes one event dispatch pass.
type DispatchResult struct {
	Matched    int `json:"matched"`
	Delivered  int `json:"delivered"`
	Suppressed int `json:"suppressed"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
}

func (t Target) Normalize() Target {
	normalized := Target{
		BridgeID:       strings.TrimSpace(t.BridgeID),
		CanonicalRoute: strings.TrimSpace(t.CanonicalRoute),
		DisplayName:    strings.TrimSpace(t.DisplayName),
		DeliveryMode:   t.DeliveryMode.Normalize(),
	}
	if normalized.DeliveryMode == "" {
		normalized.DeliveryMode = bridgepkg.DeliveryModeDirectSend
	}
	return normalized
}

func (t Target) StableHash() string {
	normalized := t.Normalize()
	if normalized.BridgeID == "" && normalized.CanonicalRoute == "" && normalized.DisplayName == "" {
		return "none"
	}
	payload := struct {
		BridgeID       string                 `json:"bridge_id"`
		CanonicalRoute string                 `json:"canonical_route,omitempty"`
		DisplayName    string                 `json:"display_name,omitempty"`
		DeliveryMode   bridgepkg.DeliveryMode `json:"delivery_mode,omitempty"`
	}{
		BridgeID:       normalized.BridgeID,
		CanonicalRoute: normalized.CanonicalRoute,
		DisplayName:    normalized.DisplayName,
		DeliveryMode:   normalized.DeliveryMode,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "none"
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func (t Target) Validate() error {
	normalized := t.Normalize()
	if normalized.BridgeID == "" {
		return fmt.Errorf("%w: target bridge_id is required", ErrInvalidPreset)
	}
	if normalized.CanonicalRoute == "" && normalized.DisplayName == "" {
		return fmt.Errorf("%w: target canonical_route or display_name is required", ErrInvalidPreset)
	}
	if err := normalized.DeliveryMode.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPreset, err)
	}
	return nil
}

func (p Preset) Normalize() Preset {
	normalized := p
	normalized.Name = normalizePresetName(p.Name)
	normalized.Events = normalizePresetEvents(p.Events)
	normalized.Targets = normalizePresetTargets(p.Targets)
	normalized.Filter = strings.TrimSpace(p.Filter)
	normalized.DefaultHash = strings.TrimSpace(p.DefaultHash)
	normalized.CreatedAt = p.CreatedAt.UTC()
	normalized.UpdatedAt = p.UpdatedAt.UTC()
	return normalized
}

func (p Preset) Validate() error {
	normalized := p.Normalize()
	if normalized.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidPreset)
	}
	if len(normalized.Events) == 0 {
		return fmt.Errorf("%w: events are required", ErrInvalidPreset)
	}
	for _, event := range normalized.Events {
		if err := ValidateEventPattern(event); err != nil {
			return err
		}
	}
	for _, target := range normalized.Targets {
		if err := target.Validate(); err != nil {
			return err
		}
	}
	if _, err := CompileFilter(normalized.Filter); err != nil {
		return err
	}
	if normalized.BuiltIn {
		if normalized.DefaultVersion == "" {
			return fmt.Errorf("%w: built-in default_version is required", ErrInvalidPreset)
		}
		if normalized.DefaultHash == "" {
			return fmt.Errorf("%w: built-in default_hash is required", ErrInvalidPreset)
		}
	}
	return nil
}

func (q Query) Normalize() Query {
	normalized := q
	normalized.Name = normalizePresetName(q.Name)
	if normalized.Limit < 0 {
		normalized.Limit = 0
	}
	return normalized
}

func (r CreateRequest) Normalize(now time.Time) (Preset, error) {
	createdAt := now.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	preset := Preset{
		Name:      normalizePresetName(r.Name),
		Events:    normalizePresetEvents(r.Events),
		Targets:   normalizePresetTargets(r.Targets),
		Filter:    strings.TrimSpace(r.Filter),
		Enabled:   r.Enabled,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	if err := preset.Validate(); err != nil {
		return Preset{}, err
	}
	return preset, nil
}

func (r UpdateRequest) HasMutableField() bool {
	return r.Events != nil || r.Targets != nil || r.Filter != nil || r.Enabled != nil
}

func (e Event) Normalize(now time.Time) Event {
	normalized := e
	normalized.ID = strings.TrimSpace(e.ID)
	normalized.Type = strings.TrimSpace(e.Type)
	normalized.WorkspaceID = strings.TrimSpace(e.WorkspaceID)
	normalized.AgentName = strings.TrimSpace(e.AgentName)
	normalized.Provider = strings.TrimSpace(e.Provider)
	normalized.TaskID = strings.TrimSpace(e.TaskID)
	normalized.RunID = strings.TrimSpace(e.RunID)
	normalized.Summary = strings.TrimSpace(e.Summary)
	if normalized.Outcome == "" {
		normalized.Outcome = eventspkg.OutcomeFor(normalized.Type)
	}
	if normalized.Timestamp.IsZero() {
		normalized.Timestamp = now.UTC()
	} else {
		normalized.Timestamp = normalized.Timestamp.UTC()
	}
	normalized.Payload = json.RawMessage(strings.TrimSpace(string(e.Payload)))
	return normalized
}

func (e Event) Validate() error {
	normalized := e.Normalize(time.Now())
	if normalized.ID == "" {
		return fmt.Errorf("%w: event id is required", ErrInvalidPreset)
	}
	if normalized.Type == "" {
		return fmt.Errorf("%w: event type is required", ErrInvalidPreset)
	}
	if normalized.Sequence <= 0 {
		return fmt.Errorf("%w: event sequence must be greater than zero", ErrInvalidPreset)
	}
	if !eventspkg.ValidOutcome(string(normalized.Outcome)) || normalized.Outcome == "" {
		return fmt.Errorf("%w: unsupported event outcome %q", ErrInvalidPreset, normalized.Outcome)
	}
	return nil
}

func BuiltInPresets(now time.Time) []Preset {
	ref := now.UTC()
	if ref.IsZero() {
		ref = time.Now().UTC()
	}
	items := []Preset{
		{
			Name:           BuiltInTaskTerminal,
			Events:         []string{BuiltInTaskRunPattern},
			Enabled:        false,
			BuiltIn:        true,
			DefaultVersion: BuiltInDefaultVersion,
			CreatedAt:      ref,
			UpdatedAt:      ref,
		},
		{
			Name: BuiltInSessionUnhealthy,
			Events: []string{
				eventspkg.SessionUnhealthy,
				eventspkg.SessionHung,
				eventspkg.SessionRecovered,
			},
			Enabled:        false,
			BuiltIn:        true,
			DefaultVersion: BuiltInDefaultVersion,
			CreatedAt:      ref,
			UpdatedAt:      ref,
		},
		{
			Name: BuiltInProviderFailure,
			Events: []string{
				eventspkg.ProviderAuthRequired,
				eventspkg.ProviderRateLimited,
				eventspkg.ProviderPermissionDenied,
				eventspkg.ProviderUnavailable,
			},
			Enabled:        false,
			BuiltIn:        true,
			DefaultVersion: BuiltInDefaultVersion,
			CreatedAt:      ref,
			UpdatedAt:      ref,
		},
	}
	for index := range items {
		items[index] = items[index].Normalize()
		items[index].DefaultHash = MutableHash(items[index])
	}
	return items
}

func ValidateEventPattern(pattern string) error {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return fmt.Errorf("%w: event pattern is required", ErrInvalidPreset)
	}
	if strings.Count(trimmed, "*") > 1 {
		return fmt.Errorf("%w: event pattern %q may contain at most one wildcard", ErrInvalidPreset, trimmed)
	}
	if strings.Contains(trimmed, "*") {
		if !strings.HasSuffix(trimmed, "*") {
			return fmt.Errorf("%w: event pattern %q only supports suffix wildcards", ErrInvalidPreset, trimmed)
		}
		prefix := strings.TrimSuffix(trimmed, "*")
		if prefix == "" || !strings.Contains(prefix, ".") {
			return fmt.Errorf("%w: event pattern %q has no public event family", ErrInvalidPreset, trimmed)
		}
		return eventspkg.ValidatePublicName(prefix)
	}
	if err := eventspkg.ValidatePublicName(trimmed); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPreset, err)
	}
	return nil
}

func MutableHash(p Preset) string {
	normalized := p.Normalize()
	payload := struct {
		Name    string   `json:"name"`
		Events  []string `json:"events"`
		Targets []Target `json:"targets"`
		Filter  string   `json:"filter"`
		Enabled bool     `json:"enabled"`
	}{
		Name:    normalized.Name,
		Events:  normalized.Events,
		Targets: normalized.Targets,
		Filter:  normalized.Filter,
		Enabled: normalized.Enabled,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func ApplyDefaultDrift(p Preset) Preset {
	normalized := p.Normalize()
	if normalized.BuiltIn && normalized.DefaultHash != "" {
		normalized.UserModified = MutableHash(normalized) != normalized.DefaultHash
	}
	return normalized
}

func normalizePresetName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizePresetEvents(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	slices.Sort(normalized)
	return normalized
}

func normalizePresetTargets(values []Target) []Target {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]Target, 0, len(values))
	for _, value := range values {
		target := value.Normalize()
		if target.BridgeID == "" && target.CanonicalRoute == "" && target.DisplayName == "" {
			continue
		}
		normalized = append(normalized, target)
	}
	slices.SortFunc(normalized, func(a Target, b Target) int {
		for _, cmp := range []int{
			strings.Compare(a.BridgeID, b.BridgeID),
			strings.Compare(a.CanonicalRoute, b.CanonicalRoute),
			strings.Compare(a.DisplayName, b.DisplayName),
			strings.Compare(string(a.DeliveryMode), string(b.DeliveryMode)),
		} {
			if cmp != 0 {
				return cmp
			}
		}
		return 0
	})
	return normalized
}
