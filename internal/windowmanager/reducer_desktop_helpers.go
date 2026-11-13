package windowmanager

import "fmt"

func setDesktopOrders(s *Snapshot) {
	for i := range s.Desktops {
		s.Desktops[i].Order = i
	}
}
func desktopEmpty(d Desktop) bool { return len(d.Tabs) == 0 }

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
