package windowmanager

func (r *reducer) markState(snapshot *Snapshot) {
	for _, desktop := range snapshot.Desktops {
		r.changes.desktop(desktop.ID)
		for _, group := range desktop.Groups {
			r.changes.group(group.ID)
			markNodeChanges(group.Root, &r.changes)
		}
	}
	for windowID := range snapshot.Windows {
		r.changes.window(windowID)
	}
}

func markNodeChanges(node LayoutNode, changes *changeBuilder) {
	changes.node(node.ID)
	for _, child := range node.Children {
		markNodeChanges(child, changes)
	}
}
