package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/auth/tui/styles"
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

// View renders the layout
func (l *LayoutComponent) View() string {
	if l.Width <= 0 || l.Height <= 0 {
		return ""
	}

	// Calculate available dimensions
	availableHeight := l.Height
	availableWidth := l.Width

	// Reserve space for header
	headerHeight := 0
	if l.ShowHeader {
		headerHeight = 2 // Title + spacing
		availableHeight -= headerHeight
	}

	// Reserve space for footer/status bar
	footerHeight := 0
	if l.ShowFooter {
		footerHeight = 1
		availableHeight -= footerHeight
	}

	// Reserve space for error component
	errorHeight := l.Error.Height()
	availableHeight -= errorHeight

	// Calculate content area
	contentWidth := availableWidth
	sidebarWidth := 0

	if l.ShowSidebar {
		sidebarWidth = l.SidebarWidth
		contentWidth -= sidebarWidth
	}

	// Build layout sections
	var sections []string

	// Header
	if l.ShowHeader {
		header := l.renderHeader()
		sections = append(sections, header)
	}

	// Error component
	if l.Error.Error != nil {
		sections = append(sections, l.Error.View())
	}

	// Main content area
	mainContent := l.renderMainContent(contentWidth, availableHeight)
	if l.ShowSidebar {
		sidebar := l.renderSidebar(sidebarWidth, availableHeight)
		mainContent = lipgloss.JoinHorizontal(lipgloss.Top, mainContent, sidebar)
	}
	sections = append(sections, mainContent)

	// Footer/Status bar
	if l.ShowFooter {
		footer := l.StatusBar.View()
		sections = append(sections, footer)
	}

	// Join all sections
	layout := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Overlay help if visible
	if l.Help.Visible {
		helpView := l.Help.View()
		layout = lipgloss.Place(l.Width, l.Height, lipgloss.Center, lipgloss.Center, helpView)
	}

	return layout
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
	if l.Content == "" {
		return ""
	}

	contentStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(0, 1)

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

	return width, height
}

// SetSize is a helper method for StatusBarComponent
func (s StatusBarComponent) SetSize(width int) StatusBarComponent {
	s.Width = width
	return s
}
