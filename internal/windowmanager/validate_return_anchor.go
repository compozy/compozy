package windowmanager

import (
	"errors"
	"fmt"
	"strings"
)

func (v *snapshotValidator) validateReturnAnchor(windowID WindowID, anchor *ReturnAnchor, path string) {
	if anchor == nil {
		return
	}
	if anchor.SourceRevision > Revision(MaxWireRevision) {
		v.add(
			"topology.return_anchor_source_revision",
			path+".source_revision",
			"return anchor source revision exceeds the JSON-safe limit",
		)
	}
	if anchor.GroupID != nil && anchor.SourceGroup == nil {
		v.add(
			"topology.return_anchor_source_group_required",
			path+".source_group",
			"tiled return anchor requires its exact source group",
		)
		return
	}
	if anchor.SourceGroup == nil {
		return
	}
	source := anchor.SourceGroup
	if anchor.GroupID == nil || *anchor.GroupID != source.ID {
		v.add(
			"topology.return_anchor_group_identity",
			path+".source_group.id",
			"source group ID must match the anchored group",
		)
	}
	v.validateReturnAnchorSourceGroup(*source, path+".source_group")
	if countNodeWindow(source.Root, windowID) != 1 {
		v.add(
			"topology.return_anchor_window_membership",
			path+".source_group.root",
			"source group must contain the anchored window exactly once",
		)
	}
}

func (v *snapshotValidator) validateReturnAnchorSourceGroup(source LayoutGroup, path string) {
	err := ValidateSnapshot(sourceGroupSnapshot(source))
	if err == nil {
		return
	}
	var topologyErr *TopologyError
	if !errors.As(err, &topologyErr) {
		v.add("topology.return_anchor_source_group", path, err.Error())
		return
	}
	for _, diagnostic := range topologyErr.Diagnostics {
		v.add(
			diagnostic.Code,
			returnAnchorSourceGroupDiagnosticPath(path, diagnostic.Path),
			diagnostic.Message,
		)
	}
}

func returnAnchorSourceGroupDiagnosticPath(path, diagnosticPath string) string {
	const syntheticGroupPath = "desktops[0].groups[0]"
	if diagnosticPath == syntheticGroupPath {
		return path
	}
	if suffix, found := strings.CutPrefix(diagnosticPath, syntheticGroupPath+"."); found {
		return fmt.Sprintf("%s.%s", path, suffix)
	}
	// The synthetic snapshot creates window records solely so the canonical
	// topology validator can inspect this group. Any record-level finding still
	// belongs to the only public source-group field, its root.
	return path + ".root"
}

func countNodeWindow(node LayoutNode, windowID WindowID) int {
	switch node.Kind {
	case NodeKindLeaf:
		if node.WindowID != nil && *node.WindowID == windowID {
			return 1
		}
		return 0
	case NodeKindStack:
		count := 0
		for _, candidate := range node.WindowIDs {
			if candidate == windowID {
				count++
			}
		}
		return count
	case NodeKindSplit:
		count := 0
		for _, child := range node.Children {
			count += countNodeWindow(child, windowID)
		}
		return count
	default:
		return 0
	}
}
