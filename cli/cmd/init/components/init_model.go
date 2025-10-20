package components

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/tui/components"
)

// InitModel is the Bubble Tea model that combines ASCII header and Huh form
type InitModel struct {
	form     *huh.Form
	viewport viewport.Model
	width    int
	height   int
	quitting bool
	header   string
	formData *ProjectFormData
}

// NewInitModel creates a new init model with header and form
func NewInitModel(formData *ProjectFormData) *InitModel {
	form := NewProjectForm(formData)
	// Create viewport for scrolling
	vp := viewport.New(80, 20)
	vp.SetContent("")
	vp.MouseWheelEnabled = true // Enable mouse wheel scrolling
	return &InitModel{
		form:     form,
		viewport: vp,
		formData: formData,
		width:    80,
		height:   24,
	}
}

// Init implements tea.Model
func (m *InitModel) Init() tea.Cmd {
	// Initialize form and request window size
	return tea.Batch(
		m.form.Init(),
		tea.WindowSize(),
	)
}

// Update implements tea.Model
func (m *InitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if quitCmd := m.handleUpdateMessage(msg); quitCmd != nil {
		return m, quitCmd
	}
	if cmd := m.updateFormState(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.viewport.SetContent(m.renderFormContent())
	viewportModel, viewportCmd := m.viewport.Update(msg)
	m.viewport = viewportModel
	cmds = append(cmds, viewportCmd)
	if quitCmd := m.checkFormCompletion(); quitCmd != nil {
		cmds = append(cmds, quitCmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *InitModel) handleUpdateMessage(msg tea.Msg) tea.Cmd {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.handleWindowResize(typed)
	case tea.KeyMsg:
		if typed.String() == "ctrl+c" {
			m.quitting = true
			return tea.Quit
		}
	}
	return nil
}

func (m *InitModel) handleWindowResize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height
	header, err := components.RenderASCIIHeader(m.width)
	if err != nil {
		m.header = "Compozy CLI"
	} else {
		m.header = header
	}
	headerLines := countLines(m.header)
	availableHeight := max(m.height-headerLines-4, 10)
	m.viewport.Width = m.width
	m.viewport.Height = availableHeight
	m.form.WithWidth(m.width - 4)
	m.viewport.SetContent(m.renderFormContent())
}

func (m *InitModel) updateFormState(msg tea.Msg) tea.Cmd {
	form, cmd := m.form.Update(msg)
	if updated, ok := form.(*huh.Form); ok {
		m.form = updated
	}
	return cmd
}

func (m *InitModel) checkFormCompletion() tea.Cmd {
	switch m.form.State {
	case huh.StateCompleted, huh.StateAborted:
		m.quitting = true
		return tea.Quit
	default:
		return nil
	}
}

// View implements tea.Model
func (m *InitModel) View() string {
	if m.quitting {
		return ""
	}
	// Generate header if not cached or width changed
	if m.header == "" {
		header, err := components.RenderASCIIHeader(m.width)
		if err != nil {
			m.header = "Compozy CLI"
		} else {
			m.header = header
		}
	}
	// Create a separator
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#333333")).
		Width(m.width).
		Align(lipgloss.Left).
		Render("─────────────────────────────────────────")
	// Compose the view: header, separator, then viewport
	return fmt.Sprintf("%s\n%s\n\n%s", m.header, separator, m.viewport.View())
}

// renderFormContent renders the form content for the viewport
func (m *InitModel) renderFormContent() string {
	// Create a container for the form with proper width
	formContainer := lipgloss.NewStyle().
		Width(m.width - 4).
		Align(lipgloss.Left)
	// Render form within the container
	return formContainer.Render(m.form.View())
}

// countLines counts the number of lines in a string
func countLines(s string) int {
	if s == "" {
		return 0
	}
	lines := 1
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines++
		}
	}
	return lines
}

// IsCompleted returns true if the form was completed successfully
func (m *InitModel) IsCompleted() bool {
	return m.form.State == huh.StateCompleted
}

// IsCanceled returns true if the form was canceled
func (m *InitModel) IsCanceled() bool {
	return m.form.State == huh.StateAborted || (m.quitting && !m.IsCompleted())
}
