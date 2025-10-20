package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/tui/styles"
)

// LayoutComponent provides a consistent layout system
type LayoutComponent struct {
	Width  int
	Height int
	Title  string

	// Layout regions
	Header  string
	Content string
	Footer  string
	Sidebar string

	// Layout options
	ShowHeader   bool
	ShowFooter   bool
	ShowSidebar  bool
	SidebarWidth int

	// Components
	StatusBar StatusBarComponent
	Help      HelpComponent
	Error     ErrorComponent
}

// NewLayoutComponent creates a new layout component
func NewLayoutComponent(title string) LayoutComponent {
	return LayoutComponent{
		Title:        title,
		ShowHeader:   true,
		ShowFooter:   true,
		ShowSidebar:  false,
		SidebarWidth: 30,
		StatusBar:    NewStatusBar(0),
		Help:         NewHelpComponent(),
		Error:        NewErrorComponent(nil, false),
	}
}

// SetSize sets the layout size
func (l *LayoutComponent) SetSize(width, height int) *LayoutComponent {
	l.Width = width
	l.Height = height
	l.StatusBar = l.StatusBar.SetSize(width)
	l.Help = l.Help.SetSize(width, height)
	return l
}

// SetContent sets the main content
func (l *LayoutComponent) SetContent(content string) *LayoutComponent {
	l.Content = content
	return l
}

// SetHeader sets the header content
func (l *LayoutComponent) SetHeader(header string) *LayoutComponent {
	l.Header = header
	return l
}

// SetFooter sets the footer content
func (l *LayoutComponent) SetFooter(footer string) *LayoutComponent {
	l.Footer = footer
	return l
}

// SetSidebar sets the sidebar content
func (l *LayoutComponent) SetSidebar(sidebar string) *LayoutComponent {
	l.Sidebar = sidebar
	return l
}

// ShowSidebarWith shows sidebar with content
func (l *LayoutComponent) ShowSidebarWith(content string) *LayoutComponent {
	l.Sidebar = content
	l.ShowSidebar = true
	return l
}

// HideSidebar hides the sidebar
func (l *LayoutComponent) HideSidebar() *LayoutComponent {
	l.ShowSidebar = false
	return l
}

// SetError sets an error message
func (l *LayoutComponent) SetError(err error, retryable bool) *LayoutComponent {
	l.Error = NewErrorComponent(err, retryable)
	return l
}

// ClearError clears the error message
func (l *LayoutComponent) ClearError() *LayoutComponent {
	l.Error = NewErrorComponent(nil, false)
	return l
}

// Update handles layout updates
func (l *LayoutComponent) Update(msg tea.Msg) (*LayoutComponent, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Update size
	if windowMsg, ok := msg.(tea.WindowSizeMsg); ok {
		l = l.SetSize(windowMsg.Width, windowMsg.Height)
	}

	// Update child components
	l.StatusBar, cmd = l.StatusBar.Update(msg)
	cmds = append(cmds, cmd)

	l.Help, cmd = l.Help.Update(msg)
	cmds = append(cmds, cmd)

	l.Error, cmd = l.Error.Update(msg)
	cmds = append(cmds, cmd)

	return l, tea.Batch(cmds...)
}

type layoutMetrics struct {
	availableHeight int
	contentWidth    int
	sidebarWidth    int
}

// View renders the layout
func (l *LayoutComponent) View() string {
	if l.Width <= 0 || l.Height <= 0 {
		return ""
	}

	metrics := l.calculateLayoutMetrics()
	sections := l.collectLayoutSections(metrics)
	layout := lipgloss.JoinVertical(lipgloss.Left, sections...)

	if l.Help.Visible {
		helpView := l.Help.View()
		layout = lipgloss.Place(l.Width, l.Height, lipgloss.Center, lipgloss.Center, helpView)
	}

	return layout
}

func (l *LayoutComponent) calculateLayoutMetrics() layoutMetrics {
	availableHeight := l.Height
	availableWidth := l.Width

	if l.ShowHeader {
		availableHeight -= 2
	}
	if l.ShowFooter {
		availableHeight--
	}

	availableHeight -= l.Error.Height()
	if availableHeight < 0 {
		availableHeight = 0
	}
	if availableWidth < 0 {
		availableWidth = 0
	}

	contentWidth := availableWidth
	sidebarWidth := 0
	if l.ShowSidebar {
		sidebarWidth = l.SidebarWidth
		contentWidth -= sidebarWidth
		if contentWidth < 0 {
			contentWidth = 0
		}
	}

	return layoutMetrics{
		availableHeight: availableHeight,
		contentWidth:    contentWidth,
		sidebarWidth:    sidebarWidth,
	}
}

func (l *LayoutComponent) collectLayoutSections(metrics layoutMetrics) []string {
	sections := make([]string, 0, 4)

	if l.ShowHeader {
		sections = append(sections, l.renderHeader())
	}

	if l.Error.Error != nil {
		sections = append(sections, l.Error.View())
	}

	mainContent := l.renderMainContent(metrics.contentWidth, metrics.availableHeight)
	if l.ShowSidebar {
		sidebar := l.renderSidebar(metrics.sidebarWidth, metrics.availableHeight)
		mainContent = lipgloss.JoinHorizontal(lipgloss.Top, mainContent, sidebar)
	}
	sections = append(sections, mainContent)

	if l.ShowFooter {
		sections = append(sections, l.StatusBar.View())
	}

	return sections
}

// renderHeader renders the header section
func (l *LayoutComponent) renderHeader() string {
	if l.Header != "" {
		return l.Header
	}

	title := styles.RenderTitle(l.Title)
	return title + "\n"
}

// renderMainContent renders the main content area
func (l *LayoutComponent) renderMainContent(width, height int) string {
	contentStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(0, 1)

	if l.Content == "" {
		// Return empty space with consistent dimensions to match GetContentSize calculations
		return contentStyle.Render("")
	}

	return contentStyle.Render(l.Content)
}

// renderSidebar renders the sidebar
func (l *LayoutComponent) renderSidebar(width, height int) string {
	if l.Sidebar == "" {
		return ""
	}

	sidebarStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(styles.Border).
		Padding(0, 1)

	return sidebarStyle.Render(l.Sidebar)
}

// GetContentSize returns the available content size
func (l *LayoutComponent) GetContentSize() (width, height int) {
	width = l.Width
	height = l.Height

	// Subtract header
	if l.ShowHeader {
		height -= 2
	}

	// Subtract footer
	if l.ShowFooter {
		height--
	}

	// Subtract error component
	height -= l.Error.Height()

	// Subtract sidebar
	if l.ShowSidebar {
		width -= l.SidebarWidth
	}

	// Subtract horizontal padding applied in renderMainContent
	width -= 2

	return width, height
}
