package windowmanager

func layoutGroupsOverlap(groups []LayoutGroup) bool {
	for left := range groups {
		for right := left + 1; right < len(groups); right++ {
			if rectsOverlap(groups[left].Frame, groups[right].Frame) {
				return true
			}
		}
	}
	return false
}

func reflowLayoutGroupFrames(groups []LayoutGroup) {
	if len(groups) == 0 {
		return
	}
	width := 1 / float64(len(groups))
	for index := range groups {
		groups[index].Frame = NormalizedRect{
			X:      float64(index) * width,
			Width:  width,
			Height: 1,
		}
	}
}

func setDesktopOrders(snapshot *Snapshot) {
	for index := range snapshot.Desktops {
		snapshot.Desktops[index].Order = index
	}
}
