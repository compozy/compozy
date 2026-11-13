package windowmanager

import (
	"fmt"
	"math"
	"slices"
)

func (c *snapshotConverter) mergeIslands(desktopID DesktopID, groups []LayoutGroupV5) (LayoutNodeV5, bool) {
	ordinal := 0
	var merge func([]LayoutGroupV5) (LayoutNodeV5, bool)
	merge = func(groups []LayoutGroupV5) (LayoutNodeV5, bool) {
		if len(groups) == 1 {
			return groups[0].Root, true
		}
		for _, axis := range []Axis{AxisHorizontal, AxisVertical} {
			cuts := []float64{}
			for _, g := range groups {
				if axis == AxisHorizontal {
					cuts = append(cuts, g.Frame.X, g.Frame.X+g.Frame.Width)
				} else {
					cuts = append(cuts, g.Frame.Y, g.Frame.Y+g.Frame.Height)
				}
			}
			slices.Sort(cuts)
			for _, cut := range slices.Compact(cuts) {
				left, right := []LayoutGroupV5{}, []LayoutGroupV5{}
				crossing := false
				for _, g := range groups {
					start, extent := g.Frame.X, g.Frame.Width
					if axis == AxisVertical {
						start, extent = g.Frame.Y, g.Frame.Height
					}
					if start+extent <= cut+1e-9 {
						left = append(left, g)
					} else if start >= cut-1e-9 {
						right = append(right, g)
					} else {
						crossing = true
						break
					}
				}
				if crossing || len(left) == 0 || len(right) == 0 {
					continue
				}
				a, ok := merge(left)
				if !ok {
					continue
				}
				b, ok := merge(right)
				if !ok {
					continue
				}
				extent := func(gs []LayoutGroupV5) float64 {
					lo, hi := math.Inf(1), math.Inf(-1)
					for _, g := range gs {
						start, size := g.Frame.X, g.Frame.Width
						if axis == AxisVertical {
							start, size = g.Frame.Y, g.Frame.Height
						}
						lo = math.Min(lo, start)
						hi = math.Max(hi, start+size)
					}
					return hi - lo
				}
				l, r := extent(left), extent(right)
				if !finite(l+r) || l <= 0 || r <= 0 {
					return LayoutNodeV5{}, false
				}
				ordinal++
				return LayoutNodeV5{
					ID:       NodeID(c.allocate(fmt.Sprintf("merge-%s-%d", desktopID, ordinal))),
					Kind:     NodeKindSplit,
					Axis:     new(axis),
					Children: []LayoutNodeV5{a, b},
					Weights:  []float64{l / (l + r), r / (l + r)},
				}, true
			}
		}
		return LayoutNodeV5{}, false
	}
	return merge(groups)
}
