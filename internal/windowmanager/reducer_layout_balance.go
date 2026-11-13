package windowmanager

import "fmt"

func (r *reducer) balance(snapshot *Snapshot, command BalanceLayoutCommand) (bool, error) {
	if command.SplitID == nil && command.TabID == nil {
		return false, fmt.Errorf("balance target is required: %w", ErrInvalidCommand)
	}
	if command.SplitID != nil {
		node, exists := findNodeInSnapshot(snapshot, *command.SplitID)
		if !exists || node.Kind != NodeKindSplit {
			return false, fmt.Errorf("split %q: %w", *command.SplitID, ErrInvalidCommand)
		}
		if weightsEqual(node.Weights) {
			return false, nil
		}
		node.Weights = equalWeights(len(node.Children))
		r.changes.node(node.ID)
		return true, nil
	}
	for desktopIndex := range snapshot.Desktops {
		for tabIndex := range snapshot.Desktops[desktopIndex].Tabs {
			tab := &snapshot.Desktops[desktopIndex].Tabs[tabIndex]
			if tab.ID != *command.TabID {
				continue
			}
			changed := balanceNode(&tab.Root, &r.changes)
			if changed {
				r.changes.tab(tab.ID)
			}
			return changed, nil
		}
	}
	return false, fmt.Errorf("tab %q: %w", *command.TabID, ErrInvalidCommand)
}

func balanceNode(node *LayoutNode, changes *changeBuilder) bool {
	changed := false
	if node.Kind == NodeKindSplit {
		if !weightsEqual(node.Weights) {
			node.Weights = equalWeights(len(node.Children))
			changes.node(node.ID)
			changed = true
		}
		for index := range node.Children {
			if balanceNode(&node.Children[index], changes) {
				changed = true
			}
		}
	}
	return changed
}
