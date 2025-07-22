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

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update header with new width
		header, err := components.RenderASCIIHeader(m.width)
		if err != nil {
			m.header = "Compozy CLI"
		} else {
			m.header = header
		}

		// Calculate available height for form (subtract header, separator, and margins)
		headerLines := countLines(m.header)
		availableHeight := max(m.height-headerLines-4, 10)

		// Update viewport dimensions
		m.viewport.Width = m.width
		m.viewport.Height = availableHeight

		// Update form width
		m.form.WithWidth(m.width - 4) // small margin on sides

		// Force viewport to update
		m.viewport.SetContent(m.renderFormContent())

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	}

	// Process form update first
	var formCmd tea.Cmd
	form, formCmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
	cmds = append(cmds, formCmd)

	// Update viewport content after form update
	m.viewport.SetContent(m.renderFormContent())

	// Update viewport for scrolling
	vpModel, vpCmd := m.viewport.Update(msg)
	m.viewport = vpModel
	cmds = append(cmds, vpCmd)

	// Check if form is completed
	if m.form.State == huh.StateCompleted {
		m.quitting = true
		cmds = append(cmds, tea.Quit)
	}

	// Check if form is aborted
	if m.form.State == huh.StateAborted {
		m.quitting = true
		cmds = append(cmds, tea.Quit)
	}

	return m, tea.Batch(cmds...)
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
