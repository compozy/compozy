package contract

import (
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	presetspkg "github.com/compozy/agh/internal/notifications/presets"
)

// NotificationTargetPayload identifies one preset delivery target.
type NotificationTargetPayload struct {
	BridgeID       string `json:"bridge_id"`
	CanonicalRoute string `json:"canonical_route,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
	DeliveryMode   string `json:"delivery_mode,omitempty"`
}

// NotificationPresetPayload is the shared HTTP/UDS notification preset shape.
type NotificationPresetPayload struct {
	Name                   string                      `json:"name"`
	Events                 []string                    `json:"events"`
	Targets                []NotificationTargetPayload `json:"targets"`
	Filter                 string                      `json:"filter,omitempty"`
	Enabled                bool                        `json:"enabled"`
	BuiltIn                bool                        `json:"built_in"`
	DefaultVersion         string                      `json:"default_version,omitempty"`
	DefaultHash            string                      `json:"default_hash,omitempty"`
	UserModified           bool                        `json:"user_modified"`
	DefaultUpdateAvailable bool                        `json:"default_update_available"`
	CreatedAt              time.Time                   `json:"created_at"`
	UpdatedAt              time.Time                   `json:"updated_at"`
}

// NotificationPresetListResponse wraps a preset list response.
type NotificationPresetListResponse struct {
	Presets     []NotificationPresetPayload `json:"presets"`
	Total       int                         `json:"total"`
	GeneratedAt time.Time                   `json:"generated_at"`
}

// NotificationPresetResponse wraps one preset response.
type NotificationPresetResponse struct {
	Preset NotificationPresetPayload `json:"preset"`
}

// CreateNotificationPresetRequest creates one preset.
type CreateNotificationPresetRequest struct {
	Name    string                      `json:"name"`
	Events  []string                    `json:"events"`
	Targets []NotificationTargetPayload `json:"targets"`
	Filter  string                      `json:"filter,omitempty"`
	Enabled bool                        `json:"enabled"`
}

// UpdateNotificationPresetRequest mutates one preset.
type UpdateNotificationPresetRequest struct {
	Events  *[]string                    `json:"events,omitempty"`
	Targets *[]NotificationTargetPayload `json:"targets,omitempty"`
	Filter  *string                      `json:"filter,omitempty"`
	Enabled *bool                        `json:"enabled,omitempty"`
}

// ToCreatePreset converts the transport create shape into the domain request.
func (r CreateNotificationPresetRequest) ToCreateRequest() presetspkg.CreateRequest {
	return presetspkg.CreateRequest{
		Name:    strings.TrimSpace(r.Name),
		Events:  append([]string(nil), r.Events...),
		Targets: notificationTargetsFromPayloads(r.Targets),
		Filter:  strings.TrimSpace(r.Filter),
		Enabled: r.Enabled,
	}
}

// ToUpdateRequest converts the transport patch shape into the domain request.
func (r UpdateNotificationPresetRequest) ToUpdateRequest() presetspkg.UpdateRequest {
	update := presetspkg.UpdateRequest{
		Events:  r.Events,
		Filter:  r.Filter,
		Enabled: r.Enabled,
	}
	if r.Targets != nil {
		targets := notificationTargetsFromPayloads(*r.Targets)
		update.Targets = &targets
	}
	return update
}

// NotificationPresetPayloadFromDomain converts a preset into a transport payload.
func NotificationPresetPayloadFromDomain(preset presetspkg.Preset) NotificationPresetPayload {
	normalized := preset.Normalize()
	return NotificationPresetPayload{
		Name:                   normalized.Name,
		Events:                 append([]string(nil), normalized.Events...),
		Targets:                notificationTargetPayloadsFromDomain(normalized.Targets),
		Filter:                 normalized.Filter,
		Enabled:                normalized.Enabled,
		BuiltIn:                normalized.BuiltIn,
		DefaultVersion:         normalized.DefaultVersion,
		DefaultHash:            normalized.DefaultHash,
		UserModified:           normalized.UserModified,
		DefaultUpdateAvailable: normalized.DefaultUpdateAvailable,
		CreatedAt:              normalized.CreatedAt,
		UpdatedAt:              normalized.UpdatedAt,
	}
}

func notificationTargetsFromPayloads(payloads []NotificationTargetPayload) []presetspkg.Target {
	targets := make([]presetspkg.Target, 0, len(payloads))
	for _, payload := range payloads {
		targets = append(targets, presetspkg.Target{
			BridgeID:       payload.BridgeID,
			CanonicalRoute: payload.CanonicalRoute,
			DisplayName:    payload.DisplayName,
			DeliveryMode:   bridgepkg.DeliveryMode(payload.DeliveryMode),
		})
	}
	return targets
}

func notificationTargetPayloadsFromDomain(targets []presetspkg.Target) []NotificationTargetPayload {
	payloads := make([]NotificationTargetPayload, 0, len(targets))
	for _, target := range targets {
		normalized := target.Normalize()
		payloads = append(payloads, NotificationTargetPayload{
			BridgeID:       normalized.BridgeID,
			CanonicalRoute: normalized.CanonicalRoute,
			DisplayName:    normalized.DisplayName,
			DeliveryMode:   string(normalized.DeliveryMode),
		})
	}
	return payloads
}
