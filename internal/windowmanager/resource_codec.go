package windowmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

const (
	WindowLayoutResourceKind resources.ResourceKind = "window_layout"
	windowLayoutMaxBytes                            = 4 * 1024 * 1024
)

type windowLayoutResourceCodec struct{}

var _ resources.KindCodec[LayoutResource] = windowLayoutResourceCodec{}

// NewLayoutResourceCodec constructs the strict declarative window-layout codec.
func NewLayoutResourceCodec() (resources.KindCodec[LayoutResource], error) {
	return windowLayoutResourceCodec{}, nil
}

func (windowLayoutResourceCodec) Kind() resources.ResourceKind {
	return WindowLayoutResourceKind
}

func (windowLayoutResourceCodec) MaxBytes() int {
	return windowLayoutMaxBytes
}

func (windowLayoutResourceCodec) Encode(resource LayoutResource) ([]byte, error) {
	encoded, err := json.Marshal(resource)
	if err != nil {
		return nil, fmt.Errorf("window manager: encode layout resource: %w", err)
	}
	if len(encoded) > windowLayoutMaxBytes {
		return nil, fmt.Errorf(
			"%w: window_layout payload exceeds %d bytes",
			resources.ErrPayloadTooLarge,
			windowLayoutMaxBytes,
		)
	}
	return encoded, nil
}

func (windowLayoutResourceCodec) DecodeAndValidate(
	ctx context.Context,
	scope resources.ResourceScope,
	raw []byte,
) (LayoutResource, error) {
	if ctx == nil {
		return LayoutResource{}, errors.New("window manager: layout resource context is required")
	}
	if err := ctx.Err(); err != nil {
		return LayoutResource{}, err
	}
	if err := scope.Validate("window_layout.scope"); err != nil {
		return LayoutResource{}, err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return LayoutResource{}, fmt.Errorf("%w: window_layout payload is required", resources.ErrValidation)
	}
	if len(trimmed) > windowLayoutMaxBytes {
		return LayoutResource{}, fmt.Errorf(
			"%w: window_layout payload exceeds %d bytes",
			resources.ErrPayloadTooLarge,
			windowLayoutMaxBytes,
		)
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	var resource LayoutResource
	if err := decoder.Decode(&resource); err != nil {
		return LayoutResource{}, fmt.Errorf("%w: decode window_layout: %w", resources.ErrValidation, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return LayoutResource{}, fmt.Errorf("%w: trailing window_layout data: %w", resources.ErrValidation, err)
	}
	return validateLayoutResource(scope.Normalize(), resource)
}

func validateLayoutResource(
	scope resources.ResourceScope,
	resource LayoutResource,
) (LayoutResource, error) {
	resource.ID = strings.TrimSpace(resource.ID)
	resource.DisplayName = strings.TrimSpace(resource.DisplayName)
	if resource.Version != LayoutResourceVersion {
		return LayoutResource{}, fmt.Errorf(
			"%w: window_layout.version must be %d",
			resources.ErrValidation,
			LayoutResourceVersion,
		)
	}
	if resource.ID == "" || resource.DisplayName == "" {
		return LayoutResource{}, fmt.Errorf(
			"%w: window_layout id and display_name are required",
			resources.ErrValidation,
		)
	}
	if resource.AspectVariant == "" {
		resource.AspectVariant = LayoutAspectAny
	}
	switch resource.AspectVariant {
	case LayoutAspectAny, LayoutAspectLandscape, LayoutAspectPortrait:
	default:
		return LayoutResource{}, fmt.Errorf(
			"%w: window_layout.aspect_variant %q is invalid",
			resources.ErrValidation,
			resource.AspectVariant,
		)
	}
	if resource.OverflowPolicy == "" {
		resource.OverflowPolicy = LayoutOverflowStack
	}
	if resource.OverflowPolicy != LayoutOverflowStack &&
		resource.OverflowPolicy != LayoutOverflowReject {
		return LayoutResource{}, fmt.Errorf(
			"%w: window_layout.overflow_policy %q is invalid",
			resources.ErrValidation,
			resource.OverflowPolicy,
		)
	}
	participantSlots, err := canonicalLayoutParticipantSlots(resource.ParticipantSlots)
	if err != nil {
		return LayoutResource{}, err
	}
	resource.ParticipantSlots = participantSlots
	if err := validateLayoutResourceDocument(scope, cloneLayoutDocument(resource.Document)); err != nil {
		return LayoutResource{}, err
	}
	resource.Document.WorkspaceID = ""
	return CloneLayoutResource(resource), nil
}

func canonicalLayoutParticipantSlots(slots []WindowID) ([]WindowID, error) {
	canonical := make([]WindowID, len(slots))
	seenSlots := make(map[WindowID]struct{}, len(slots))
	for index, slot := range slots {
		canonicalSlot := WindowID(strings.TrimSpace(string(slot)))
		if canonicalSlot == "" {
			return nil, fmt.Errorf(
				"%w: window_layout.participant_slots[%d] is required",
				resources.ErrValidation,
				index,
			)
		}
		if _, duplicate := seenSlots[canonicalSlot]; duplicate {
			return nil, fmt.Errorf(
				"%w: window_layout participant slot %q is duplicated",
				resources.ErrValidation,
				canonicalSlot,
			)
		}
		seenSlots[canonicalSlot] = struct{}{}
		canonical[index] = canonicalSlot
	}
	return canonical, nil
}

func validateLayoutResourceDocument(scope resources.ResourceScope, document LayoutDocument) error {
	if document.Version != SnapshotVersion {
		return fmt.Errorf(
			"%w: window_layout document version must be %d",
			resources.ErrValidation,
			SnapshotVersion,
		)
	}
	validationWorkspace := WorkspaceID("window-layout-resource")
	if scope.Kind == resources.ResourceScopeKindWorkspace {
		validationWorkspace = WorkspaceID(scope.ID)
		if document.WorkspaceID != "" && document.WorkspaceID != validationWorkspace {
			return fmt.Errorf(
				"%w: window_layout workspace binding does not match scope",
				resources.ErrInvalidScopeBinding,
			)
		}
	} else if document.WorkspaceID != "" {
		return fmt.Errorf(
			"%w: global window_layout document must not bind a workspace",
			resources.ErrInvalidScopeBinding,
		)
	}
	document.WorkspaceID = validationWorkspace
	candidate := Snapshot{
		Version: document.Version, WorkspaceID: validationWorkspace,
		Desktops: document.Desktops, Windows: document.Windows, Overrides: document.Overrides,
	}
	if _, err := effectiveConfig(DefaultConfig(), candidate.Overrides); err != nil {
		return fmt.Errorf("%w: window_layout overrides: %w", resources.ErrValidation, err)
	}
	if err := ValidateSnapshot(candidate); err != nil {
		return fmt.Errorf("%w: window_layout topology: %w", resources.ErrValidation, err)
	}
	return nil
}
