package run

type uiLayoutState struct {
	mode          uiLayoutMode
	sidebarWidth  int
	timelineWidth int
	contentHeight int
}

func (m *uiModel) computeLayout(totalWidth, totalHeight int) uiLayoutState {
	if totalWidth < 80 || totalHeight < 24 {
		return uiLayoutState{
			mode:          uiLayoutResizeBlocked,
			sidebarWidth:  max(totalWidth, 1),
			timelineWidth: 0,
			contentHeight: max(totalHeight-chromeHeight, 1),
		}
	}

	contentHeight := max(totalHeight-chromeHeight, minContentHeight)
	sidebar := clamp(int(float64(totalWidth)*0.28), sidebarMinWidth, min(sidebarMaxWidth, totalWidth/2))
	main := totalWidth - sidebar
	if main < mainMinWidth {
		sidebar = max(totalWidth-mainMinWidth, sidebarMinWidth)
		main = max(totalWidth-sidebar, 1)
	}
	return uiLayoutState{
		mode:          uiLayoutSplit,
		sidebarWidth:  sidebar,
		timelineWidth: max(main, timelineMinWidth),
		contentHeight: contentHeight,
	}
}

func (m *uiModel) configureViewports(layout uiLayoutState) {
	sidebarViewportWidth := max(sidebarContentWidth(layout.sidebarWidth), 10)
	sidebarViewportHeight := max(sidebarContentHeight(layout.contentHeight), sidebarViewportMinRows)
	m.sidebarViewport.SetWidth(sidebarViewportWidth)
	m.sidebarViewport.SetHeight(sidebarViewportHeight)

	m.transcriptViewport.SetWidth(max(panelContentWidth(layout.timelineWidth), 10))
	m.transcriptViewport.SetHeight(max(layout.contentHeight-4, logViewportMinHeight))
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func truncateString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen == 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}
