package windowmanager

import "fmt"

// displaceGroupsForFrame shrinks every island the new frame overlaps to the
// part of its frame the new frame leaves free. An edge or corner drop claims
// its zone the way a tiling desktop does; an island the frame swallows whole,
// or cuts through the middle, has no single remainder and is rejected.
func displaceGroupsForFrame(groups []LayoutGroup, frame NormalizedRect) ([]GroupID, error) {
	displaced := make([]GroupID, 0)
	for index := range groups {
		group := &groups[index]
		if !rectsOverlap(group.Frame, frame) {
			continue
		}
		remainder, ok := rectRemainder(group.Frame, frame)
		if !ok {
			return nil, fmt.Errorf(
				"arrangement frame leaves island %q no room: %w",
				group.ID,
				ErrInvalidCommand,
			)
		}
		group.Frame = remainder
		displaced = append(displaced, group.ID)
	}
	return displaced, nil
}

// rectRemainder returns the largest rectangle of rect that lies outside cut.
// A cut that spans rect along one axis leaves one band; a corner cut keeps the
// larger of the two possible bands; a cut strictly inside rect has no answer.
func rectRemainder(rect, cut NormalizedRect) (NormalizedRect, bool) {
	left := rect.X
	right := rect.X + rect.Width
	top := rect.Y
	bottom := rect.Y + rect.Height
	cutLeft := cut.X
	cutRight := cut.X + cut.Width
	cutTop := cut.Y
	cutBottom := cut.Y + cut.Height
	candidates := make([]NormalizedRect, 0, 2)
	if cutLeft <= left+weightTolerance && cutRight < right-weightTolerance {
		candidates = append(
			candidates,
			NormalizedRect{X: cutRight, Y: top, Width: right - cutRight, Height: rect.Height},
		)
	}
	if cutRight >= right-weightTolerance && cutLeft > left+weightTolerance {
		candidates = append(candidates, NormalizedRect{X: left, Y: top, Width: cutLeft - left, Height: rect.Height})
	}
	if cutTop <= top+weightTolerance && cutBottom < bottom-weightTolerance {
		candidates = append(
			candidates,
			NormalizedRect{X: left, Y: cutBottom, Width: rect.Width, Height: bottom - cutBottom},
		)
	}
	if cutBottom >= bottom-weightTolerance && cutTop > top+weightTolerance {
		candidates = append(candidates, NormalizedRect{X: left, Y: top, Width: rect.Width, Height: cutTop - top})
	}
	best := NormalizedRect{}
	found := false
	for _, candidate := range candidates {
		if !validRect(candidate) || rectsOverlap(candidate, cut) {
			continue
		}
		if !found || candidate.Width*candidate.Height > best.Width*best.Height {
			best = candidate
			found = true
		}
	}
	return best, found
}
