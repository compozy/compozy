package windowmanager

import "fmt"

// detachRemainderGroups rebuilds a group without the subtree at path, keeping
// every surviving sibling run as an island pinned to its exact projected zone.
func detachRemainderGroups(group LayoutGroup, path []int, generate idGenerator) ([]LayoutGroup, error) {
	zones := make(map[NodeID]NormalizedRect)
	nodeZones(group.Root, group.Frame, zones)
	remainders := make([]LayoutGroup, 0, len(path)*2)
	if err := collectRemainderGroups(group.Root, path, zones, generate, &remainders); err != nil {
		return nil, err
	}
	return remainders, nil
}

func collectRemainderGroups(
	node LayoutNode,
	path []int,
	zones map[NodeID]NormalizedRect,
	generate idGenerator,
	output *[]LayoutGroup,
) error {
	if len(path) == 0 {
		return nil
	}
	childIndex := path[0]
	if node.Kind != NodeKindSplit || childIndex < 0 || childIndex >= len(node.Children) {
		return fmt.Errorf("detach path is invalid: %w", ErrInvalidTopology)
	}
	if childIndex > 0 {
		runGroup, err := buildRunGroup(node, 0, childIndex, zones, generate)
		if err != nil {
			return err
		}
		*output = append(*output, runGroup)
	}
	if err := collectRemainderGroups(node.Children[childIndex], path[1:], zones, generate, output); err != nil {
		return err
	}
	if childIndex+1 < len(node.Children) {
		runGroup, err := buildRunGroup(node, childIndex+1, len(node.Children), zones, generate)
		if err != nil {
			return err
		}
		*output = append(*output, runGroup)
	}
	return nil
}

func buildRunGroup(
	split LayoutNode,
	start, end int,
	zones map[NodeID]NormalizedRect,
	generate idGenerator,
) (LayoutGroup, error) {
	groupID, err := generate("group")
	if err != nil {
		return LayoutGroup{}, fmt.Errorf("generate group ID: %w", err)
	}
	if end-start == 1 {
		child := split.Children[start]
		return LayoutGroup{ID: GroupID(groupID), Frame: zones[child.ID], Root: child}, nil
	}
	if split.Axis == nil {
		return LayoutGroup{}, fmt.Errorf("split %q has no axis: %w", split.ID, ErrInvalidTopology)
	}
	nodeID, err := generate("node")
	if err != nil {
		return LayoutGroup{}, fmt.Errorf("generate split ID: %w", err)
	}
	children := append([]LayoutNode(nil), split.Children[start:end]...)
	weights := make([]float64, end-start)
	for index := range weights {
		weights[index] = splitWeightAt(split, start+index)
	}
	normalizeWeights(weights)
	axis := *split.Axis
	frame := runZone(axis, zones[split.ID], zones[children[0].ID], zones[children[len(children)-1].ID])
	root := LayoutNode{ID: NodeID(nodeID), Kind: NodeKindSplit, Axis: &axis, Children: children, Weights: weights}
	return LayoutGroup{ID: GroupID(groupID), Frame: frame, Root: root}, nil
}

func splitWeightAt(split LayoutNode, index int) float64 {
	if index < len(split.Weights) && finite(split.Weights[index]) && split.Weights[index] > 0 {
		return split.Weights[index]
	}
	return 1
}

func runZone(axis Axis, splitZone, firstZone, lastZone NormalizedRect) NormalizedRect {
	if axis == AxisHorizontal {
		return NormalizedRect{
			X:      firstZone.X,
			Y:      splitZone.Y,
			Width:  lastZone.X + lastZone.Width - firstZone.X,
			Height: splitZone.Height,
		}
	}
	return NormalizedRect{
		X:      splitZone.X,
		Y:      firstZone.Y,
		Width:  splitZone.Width,
		Height: lastZone.Y + lastZone.Height - firstZone.Y,
	}
}

// nodeZones mirrors projectNode's subdivision math keyed by node ID.
func nodeZones(node LayoutNode, frame NormalizedRect, output map[NodeID]NormalizedRect) {
	output[node.ID] = frame
	if node.Kind != NodeKindSplit {
		return
	}
	offset := 0.0
	for index, child := range node.Children {
		weight := splitWeightAt(node, index)
		childFrame := frame
		if node.Axis != nil && *node.Axis == AxisHorizontal {
			childFrame.X = frame.X + frame.Width*offset
			childFrame.Width = frame.Width * weight
		} else {
			childFrame.Y = frame.Y + frame.Height*offset
			childFrame.Height = frame.Height * weight
		}
		nodeZones(child, childFrame, output)
		offset += weight
	}
}

func nodePath(node LayoutNode, targetID NodeID) ([]int, bool) {
	if node.ID == targetID {
		return []int{}, true
	}
	for index := range node.Children {
		if suffix, found := nodePath(node.Children[index], targetID); found {
			return append([]int{index}, suffix...), true
		}
	}
	return nil, false
}

func nodeAtPath(node LayoutNode, path []int) LayoutNode {
	for _, index := range path {
		node = node.Children[index]
	}
	return node
}
