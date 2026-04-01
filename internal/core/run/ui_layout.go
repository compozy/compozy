package run

func paddedContentWidth(width int) int {
	return max(width-mainHorizontalPadding, 1)
}

func (m *uiModel) computePaneWidths(totalWidth int) (int, int) {
	sidebar := m.initialSidebarWidth(totalWidth)
	main := totalWidth - sidebar
	if main < mainMinWidth {
		main = mainMinWidth
		sidebar = totalWidth - main
		if sidebar < sidebarMinWidth {
			sidebar = sidebarMinWidth
			main = totalWidth - sidebar
		}
	}
	return sidebar, max(main, 0)
}

func (m *uiModel) initialSidebarWidth(totalWidth int) int {
	sidebar := min(max(int(float64(totalWidth)*sidebarWidthRatio), sidebarMinWidth), sidebarMaxWidth)
	if sidebar >= totalWidth {
		sidebar = totalWidth / 2
	}
	return sidebar
}

func (m *uiModel) computeContentHeight(totalHeight int) int {
	return max(totalHeight-chromeHeight, minContentHeight)
}

func (m *uiModel) configureViewports(sidebarWidth, mainWidth, contentHeight int) {
	sidebarViewportWidth := max(sidebarContentWidth(sidebarWidth), 10)
	sidebarViewportHeight := max(sidebarContentHeight(contentHeight), sidebarViewportMinRows)
	m.sidebarViewport.SetWidth(sidebarViewportWidth)
	if m.sidebarViewport.YOffset() > sidebarViewportHeight {
		m.sidebarViewport.SetYOffset(sidebarViewportHeight)
	}
	m.sidebarViewport.SetHeight(sidebarViewportHeight)

	m.viewport.SetWidth(max(paddedContentWidth(mainWidth), 10))
	m.viewport.SetHeight(max(contentHeight, logViewportMinHeight))
}

func (m *uiModel) mainDimensions() (int, int) {
	contentHeight := max(m.contentHeight, minContentHeight)
	mainWidth := m.mainWidth
	if mainWidth <= 0 {
		sidebar := min(max(int(float64(m.width)*sidebarWidthRatio), sidebarMinWidth), sidebarMaxWidth)
		mainWidth = m.width - sidebar
	}
	return max(mainWidth, 1), contentHeight
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
