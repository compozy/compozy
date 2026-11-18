package windowmanager

func (r *reducer) swapWindows(s *Snapshot, c SwapWindowsCommand) (bool, error) {
	if c.FirstWindowID == c.SecondWindowID {
		return false, nil
	}
	first, ok := findWindowPlacement(s, c.FirstWindowID)
	if !ok {
		return false, ErrWindowNotFound
	}
	second, ok := findWindowPlacement(s, c.SecondWindowID)
	if !ok {
		return false, ErrWindowNotFound
	}
	if s.Windows[c.FirstWindowID].TabID != s.Windows[c.SecondWindowID].TabID {
		return false, ErrInvalidCommand
	}
	a, _ := findNodeInSnapshot(s, first.nodeID)
	b, _ := findNodeInSnapshot(s, second.nodeID)
	a.WindowID, b.WindowID = b.WindowID, a.WindowID
	r.changes.window(c.FirstWindowID)
	r.changes.window(c.SecondWindowID)
	r.changes.node(a.ID)
	r.changes.node(b.ID)
	return true, nil
}
