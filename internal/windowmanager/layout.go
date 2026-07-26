package windowmanager

// LayoutResourceVersion is the schema version required for a declarative window_layout resource.
const LayoutResourceVersion uint32 = 1

type LayoutAspectVariant string

const (
	LayoutAspectAny       LayoutAspectVariant = "any"
	LayoutAspectLandscape LayoutAspectVariant = "landscape"
	LayoutAspectPortrait  LayoutAspectVariant = "portrait"
)

type LayoutOverflowPolicy string

const (
	LayoutOverflowStack  LayoutOverflowPolicy = "stack"
	LayoutOverflowReject LayoutOverflowPolicy = "reject"
)

// LayoutDocument is the validated raw export and replacement contract.
type LayoutDocument struct {
	Version     uint32              `json:"version"`
	WorkspaceID WorkspaceID         `json:"workspace_id"`
	Desktops    []Desktop           `json:"desktops"`
	Windows     map[WindowID]Window `json:"windows"`
	Overrides   WorkspaceConfig     `json:"overrides"`
}

// LayoutResource describes one declarative registry contribution.
type LayoutResource struct {
	Version          uint32               `json:"version"`
	ID               string               `json:"id"`
	DisplayName      string               `json:"display_name"`
	AspectVariant    LayoutAspectVariant  `json:"aspect_variant"`
	ParticipantSlots []WindowID           `json:"participant_slots,omitempty"`
	OverflowPolicy   LayoutOverflowPolicy `json:"overflow_policy"`
	Document         LayoutDocument       `json:"document"`
}

// CloneLayoutResource returns a deep copy safe for cross-package catalogs.
func CloneLayoutResource(resource LayoutResource) LayoutResource {
	resource.ParticipantSlots = append([]WindowID(nil), resource.ParticipantSlots...)
	resource.Document = cloneLayoutDocument(resource.Document)
	return resource
}

// Validation reports stable layout diagnostics.
type Validation struct {
	Valid       bool         `json:"valid"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// ReplaceLayoutRequest applies a validated document with normal revision CAS.
type ReplaceLayoutRequest struct {
	WorkspaceID      WorkspaceID
	ExpectedRevision Revision
	ClientID         *ClientID
	Actor            Actor
	Origin           string
	Document         LayoutDocument
}
