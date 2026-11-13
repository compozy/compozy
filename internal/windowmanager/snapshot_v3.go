package windowmanager

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"
)

// LegacySnapshotVersion is the last stored aggregate shape that marked the
// desktop zoom created with a purpose and owner instead of flagging the window.
const LegacySnapshotVersion uint32 = 3

const legacyFocusPurpose = "focus"

type legacyDesktopV3 struct {
	ID             DesktopID         `json:"id"`
	Name           string            `json:"name"`
	Order          int               `json:"order"`
	Purpose        string            `json:"purpose"`
	FocusOwner     *WindowID         `json:"focus_owner,omitzero"`
	Groups         []LayoutGroupV5   `json:"groups"`
	Floating       []WindowID        `json:"floating"`
	FloatingStacks []FloatingStackV5 `json:"floating_stacks"`
}

type legacySnapshotV3 struct {
	Version       uint32                `json:"version"`
	WorkspaceID   WorkspaceID           `json:"workspace_id"`
	Revision      Revision              `json:"revision"`
	Desktops      []legacyDesktopV3     `json:"desktops"`
	Windows       map[WindowID]WindowV5 `json:"windows"`
	ClosedEntries []ClosedEntryV5       `json:"closed_entries,omitempty"`
	History       json.RawMessage       `json:"history"`
	Overrides     WorkspaceConfigV5     `json:"overrides"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

// MigrateLegacySnapshotV3 chains v3 through the v4 geometry and v5 app identities.
// A focus desktop becomes a regular desktop whose owner is zoomed on it
// with its return anchor intact, exactly like a unit that zoom lifted there;
// layout history does not survive because its entries carry the old shape.
// The result carries the next revision: its content differs from what any
// client cached under the stored revision.
func MigrateLegacySnapshotV3(encoded []byte) (SnapshotV5, error) {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var legacy legacySnapshotV3
	if err := decoder.Decode(&legacy); err != nil {
		return SnapshotV5{}, fmt.Errorf("decode legacy window-manager snapshot: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return SnapshotV5{}, errors.New("decode legacy window-manager snapshot: trailing data")
	}
	if legacy.Version != LegacySnapshotVersion {
		return SnapshotV5{}, fmt.Errorf(
			"legacy window-manager snapshot version %d is not %d",
			legacy.Version,
			LegacySnapshotVersion,
		)
	}
	revision, err := NextTopologyRevision(legacy.Revision)
	if err != nil {
		return SnapshotV5{}, fmt.Errorf("legacy window-manager snapshot revision: %w", err)
	}
	snapshot := SnapshotV5{
		Version:       5,
		WorkspaceID:   legacy.WorkspaceID,
		Revision:      revision,
		Desktops:      make([]DesktopV5, 0, len(legacy.Desktops)),
		Windows:       legacy.Windows,
		ClosedEntries: legacy.ClosedEntries,
		History:       json.RawMessage(`{"undo":[],"redo":[]}`),
		Overrides:     legacy.Overrides,
		UpdatedAt:     legacy.UpdatedAt,
	}
	if snapshot.Windows == nil {
		snapshot.Windows = map[WindowID]WindowV5{}
	}
	for _, desktop := range legacy.Desktops {
		snapshot.Desktops = append(snapshot.Desktops, DesktopV5{
			ID: desktop.ID, Name: desktop.Name, Order: desktop.Order,
			Groups: desktop.Groups, Floating: desktop.Floating, FloatingStacks: desktop.FloatingStacks,
		})
		if desktop.Purpose != legacyFocusPurpose || desktop.FocusOwner == nil {
			continue
		}
		if owner, exists := snapshot.Windows[*desktop.FocusOwner]; exists && owner.DesktopID == desktop.ID {
			owner.Zoomed = true
			snapshot.Windows[*desktop.FocusOwner] = owner
		} else {
			dropEmptyLegacyFocusDesktop(&snapshot, desktop.ID)
		}
	}
	for i := range snapshot.Desktops {
		snapshot.Desktops[i].Order = i
	}
	return snapshot, nil
}
func dropEmptyLegacyFocusDesktop(snapshot *SnapshotV5, desktopID DesktopID) {
	index := -1
	for i, d := range snapshot.Desktops {
		if d.ID == desktopID {
			index = i
			break
		}
	}
	exists := index >= 0
	if !exists || len(snapshot.Desktops) == 1 {
		return
	}
	if len(
		snapshot.Desktops[index].Groups,
	)+len(
		snapshot.Desktops[index].Floating,
	)+len(
		snapshot.Desktops[index].FloatingStacks,
	) > 0 {
		return
	}
	snapshot.Desktops = slices.Delete(snapshot.Desktops, index, index+1)
}
