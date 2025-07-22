package components

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/compozy/compozy/cli/tui/models"
)

// FormWrapper wraps a Huh form with BaseModel integration
type FormWrapper struct {
	models.BaseModel
	form      *huh.Form
	canceled  bool
	completed bool
}

// NewFormWrapper creates a new form wrapper
func NewFormWrapper(ctx context.Context, form *huh.Form) *FormWrapper {
	return &FormWrapper{
		BaseModel: models.NewBaseModel(ctx, models.ModeTUI),
		form:      form,
	}
}

// Init initializes the form
func (f *FormWrapper) Init() tea.Cmd {
	return f.form.Init()
}

// Update handles form updates
func (f *FormWrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle key messages for cancellation
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c", "q":
			f.canceled = true
			return f, tea.Quit
		}
	}

	// Handle base model updates (like window size)
	f.BaseModel.Update(msg)

	// Delegate to huh form
	form, cmd := f.form.Update(msg)
	if frm, ok := form.(*huh.Form); ok {
		f.form = frm
		if f.form.State == huh.StateCompleted {
			f.completed = true
			return f, tea.Quit
		}
		if f.form.State == huh.StateAborted {
			f.canceled = true
			return f, tea.Quit
		}
	}

	return f, cmd
}

// View renders the form
func (f *FormWrapper) View() string {
	return f.form.View()
}

// IsCanceled returns whether the form was canceled
func (f *FormWrapper) IsCanceled() bool {
	return f.canceled
}

// IsCompleted returns whether the form was completed
func (f *FormWrapper) IsCompleted() bool {
	return f.completed
}

// GetForm returns the underlying huh form for accessing values
func (f *FormWrapper) GetForm() *huh.Form {
	return f.form
}
