// Package command owns session-aware slash command projection and parsing.
package command

// Lane groups commands by their runtime owner.
type Lane string

const (
	LaneBuiltin Lane = "builtin"
	LaneAgent   Lane = "agent"
	LaneSkill   Lane = "skill"
)

// Placement identifies where a command may be selected from authored text.
type Placement string

const (
	PlacementStandalone Placement = "standalone"
	PlacementInline     Placement = "inline"
)

// Source identifies the owner and scope of a projected command.
type Source struct {
	Kind   string `json:"kind"`
	ID     string `json:"id,omitempty"`
	Key    string `json:"key,omitempty"`
	Scope  string `json:"scope"`
	Origin string `json:"origin,omitempty"`
}

// SkillRef identifies one exact skill definition without exposing its path.
type SkillRef struct {
	CommandID string `json:"command_id"`
	Name      string `json:"name"`
	Source    Source `json:"source"`
}

// Descriptor is one available slash command.
type Descriptor struct {
	ID                string      `json:"id"`
	CanonicalToken    string      `json:"canonical_token"`
	DisplayName       string      `json:"display_name"`
	Description       string      `json:"description"`
	Lane              Lane        `json:"lane"`
	Source            Source      `json:"source"`
	Placements        []Placement `json:"placements"`
	InputHint         string      `json:"input_hint,omitempty"`
	Available         bool        `json:"available"`
	UnavailableReason string      `json:"unavailable_reason,omitempty"`
	Skill             *SkillRef   `json:"-"`
}

// Catalog is the deterministic command projection for one session snapshot.
type Catalog struct {
	Commands    []Descriptor
	Revision    string
	unavailable map[string]struct{}
}

// Invocation records one exact skill marker found in authored text.
type Invocation struct {
	Ref   SkillRef `json:"ref"`
	Token string   `json:"token"`
	Start int      `json:"start"`
	End   int      `json:"end"`
}

// BuiltinSpec describes one daemon-owned control command.
type BuiltinSpec struct {
	Name              string
	Description       string
	InputHint         string
	Available         bool
	UnavailableReason string
}

// AgentSpec describes one ACP-advertised control command.
type AgentSpec struct {
	Name        string
	Description string
	InputHint   string
	SourceID    string
}

// SkillSpec describes one source-qualified skill candidate.
type SkillSpec struct {
	Name        string
	Description string
	Source      Source
	Available   bool
	Qualified   bool
}
