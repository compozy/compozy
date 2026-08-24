package cmdpalette

import (
	"context"
	"encoding/json"
)

// ViewOpenRequest is the extension-facing view/open request.
type ViewOpenRequest struct {
	ViewSession string         `json:"view_session"`
	View        string         `json:"view"`
	ProfileLens ProfileLens    `json:"profile_lens"`
	Workspace   WorkspaceID    `json:"workspace"`
	Client      ClientID       `json:"client"`
	Args        map[string]any `json:"args,omitempty"`
}

// ViewEvent is the extension-facing view/event request.
type ViewEvent struct {
	ViewSession  string        `json:"view_session"`
	Handler      string        `json:"handler"`
	Args         []any         `json:"args,omitempty"`
	Revision     string        `json:"revision"`
	Seq          int64         `json:"seq"`
	Generation   uint64        `json:"generation"`
	AckEffects   []string      `json:"ack_effects,omitempty"`
	EffectResult *EffectResult `json:"effect_result,omitempty"`
}

// EffectResult correlates a host effect result with the originating effect.
type EffectResult struct {
	EffectID string          `json:"effect_id"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

// ViewFrame is one full or incremental programmable-view render.
type ViewFrame struct {
	ViewSession string       `json:"view_session"`
	Revision    string       `json:"revision"`
	InReplyTo   int64        `json:"in_reply_to,omitempty"`
	Generation  uint64       `json:"generation"`
	Payload     *ViewPayload `json:"payload,omitempty"`
	Patch       *ViewPatch   `json:"patch,omitempty"`
	Effects     []Effect     `json:"effects,omitempty"`
	Handlers    []string     `json:"handlers"`
}

// ViewCloseRequest is the extension-facing view/close request.
type ViewCloseRequest struct {
	ViewSession string      `json:"view_session"`
	ProfileLens ProfileLens `json:"profile_lens"`
	Reason      string      `json:"reason,omitempty"`
}

// SessionToken carries exactly one proof used to access a view session.
// AttachmentToken is supplied by the owning client, StreamToken by the SSE
// reader, and Extension by the owning extension's Host API call.
type SessionToken struct {
	ViewSession     string `json:"view_session"`
	StreamToken     string `json:"stream_token,omitempty"`
	AttachmentToken string `json:"-"`
	Extension       string `json:"-"`
}

// ViewSessionOpenRequest is the authenticated host request to create a session.
type ViewSessionOpenRequest struct {
	ProfileLens     ProfileLens
	Workspace       WorkspaceID
	Client          ClientID
	AttachmentToken string
	View            string
	Args            map[string]any
}

// ViewSessionOpenResult is returned to the attached client after view/open.
type ViewSessionOpenResult struct {
	ProfileLens ProfileLens  `json:"profile_lens"`
	Token       SessionToken `json:"-"`
	FirstFrame  ViewFrame    `json:"first_frame"`
}

// ViewProgramProvider calls the negotiated view.provider service.
type ViewProgramProvider interface {
	OpenProgram(context.Context, string, ViewOpenRequest) (ViewFrame, uint64, error)
	HandleProgramEvent(context.Context, ProfileLens, WorkspaceID, string, ViewEvent) (*ViewFrame, error)
	CloseProgram(context.Context, ProfileLens, WorkspaceID, string, ViewCloseRequest) error
}
