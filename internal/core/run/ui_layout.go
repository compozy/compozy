package run

import "github.com/charmbracelet/lipgloss"

func (m *uiModel) computePaneWidths(totalWidth int) (int, int) {
	sidebar := m.initialSidebarWidth(totalWidth)
	main := totalWidth - sidebar
	if main < mainMinWidth {
		main = mainMinWidth
		if main >= totalWidth {
			main = totalWidth - sidebarMinWidth
		}
		sidebar = totalWidth - main
		if sidebar < sidebarMinWidth {
			sidebar = sidebarMinWidth
			main = totalWidth - sidebar
		}
	}
	if main < 0 {
		main = 0
	}
	return sidebar, main
}

func (m *uiModel) initialSidebarWidth(totalWidth int) int {
	sidebar := int(float64(totalWidth) * sidebarWidthRatio)
	if sidebar < sidebarMinWidth {
		sidebar = sidebarMinWidth
	}
	if sidebar > sidebarMaxWidth {
		sidebar = sidebarMaxWidth
	}
	if sidebar >= totalWidth {
		sidebar = totalWidth / 2
	}
	return sidebar
}

func (m *uiModel) computeContentHeight(totalHeight int) int {
	content := totalHeight - chromeHeight
	if content < minContentHeight {
		return minContentHeight
	}
	return content
}

func (m *uiModel) configureViewports(sidebarWidth, mainWidth, contentHeight int) {
	sidebarViewportWidth := sidebarWidth - sidebarChromeWidth
	if sidebarViewportWidth < 10 {
		sidebarViewportWidth = 10
	}
	sidebarViewportHeight := contentHeight - sidebarChromeHeight
	if sidebarViewportHeight < sidebarViewportMinRows {
		sidebarViewportHeight = sidebarViewportMinRows
	}
	m.sidebarViewport.Width = sidebarViewportWidth
	if m.sidebarViewport.YOffset > sidebarViewportHeight {
		m.sidebarViewport.SetYOffset(sidebarViewportHeight)
	}
	m.sidebarViewport.Height = sidebarViewportHeight

	mainViewportWidth := mainWidth - mainHorizontalPadding
	if mainViewportWidth < 10 {
		mainViewportWidth = 10
	}
	m.viewport.Width = mainViewportWidth
	if contentHeight < logViewportMinHeight {
		m.viewport.Height = logViewportMinHeight
	} else {
		m.viewport.Height = contentHeight
	}
}

func (m *uiModel) availableLogHeight(contentHeight int, metaBlock, logsHeader string) int {
	usedHeight := lipgloss.Height(metaBlock) + lipgloss.Height(logsHeader)
	available := contentHeight - usedHeight
	if available < logViewportMinHeight {
		return logViewportMinHeight
	}
	return available
}

func (m *uiModel) mainDimensions() (int, int) {
	contentHeight := m.contentHeight
	if contentHeight < minContentHeight {
		contentHeight = minContentHeight
	}
	mainWidth := m.mainWidth
	if mainWidth <= 0 {
		mainWidth = int(float64(m.width) * (1 - sidebarWidthRatio))
	}
	if mainWidth < mainMinWidth {
		mainWidth = mainMinWidth
	}
	return mainWidth, contentHeight
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
