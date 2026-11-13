package windowmanager

import "fmt"

func setDesktopOrders(s *Snapshot) {
	for i := range s.Desktops {
		s.Desktops[i].Order = i
	}
}
func placeholderTab(s *Snapshot, tab Tab) bool {
	ids := nodeWindowIDs(tab.Root)
	return len(ids) == 1 && s.Windows[ids[0]].App == newTabApp
}

func desktopEmpty(s *Snapshot, d Desktop) bool {
	for _, tab := range d.Tabs {
		if !placeholderTab(s, tab) {
			return false
		}
	}
	return true
}

func nextDesktopName(s *Snapshot) string {
	for n := 1; ; n++ {
		name := fmt.Sprintf("Desktop %d", n)
		used := false
		for _, d := range s.Desktops {
			if d.Name == name {
				used = true
				break
			}
		}
		if !used {
			return name
		}
	}
}
