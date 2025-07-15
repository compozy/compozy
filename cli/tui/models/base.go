package models

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

// Mode represents the output mode for CLI commands
type Mode string

const (
	// ModeTUI represents interactive TUI mode
	ModeTUI Mode = "tui"
	// ModeJSON represents non-interactive JSON output mode
	ModeJSON Mode = "json"
)

// BaseModel provides common functionality for all TUI models
type BaseModel struct {
	ctx      context.Context
	mode     Mode
	width    int
	height   int
	ready    bool
	quitting bool
	err      error
}

// NewBaseModel creates a new base model
func NewBaseModel(ctx context.Context, mode Mode) BaseModel {
	return BaseModel{
		ctx:  ctx,
		mode: mode,
	}
}

// Context returns the context
func (m BaseModel) Context() context.Context {
	return m.ctx
}

// Mode returns the current mode
func (m BaseModel) Mode() Mode {
	return m.mode
}

// Size returns the terminal size
func (m BaseModel) Size() (width, height int) {
	return m.width, m.height
}

// IsReady returns whether the model is ready
func (m BaseModel) IsReady() bool {
	return m.ready
}

// IsQuitting returns whether the model is quitting
func (m BaseModel) IsQuitting() bool {
	return m.quitting
}

// Error returns any error that occurred
func (m BaseModel) Error() error {
	return m.err
}

// SetSize sets the terminal size
func (m *BaseModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.ready = true
}

// SetError sets an error
func (m *BaseModel) SetError(err error) {
	m.err = err
}

// Quit marks the model as quitting
func (m *BaseModel) Quit() {
	m.quitting = true
}

// Update handles common messages for all models
func (m *BaseModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Quit()
			return tea.Quit
		}
	}
	return nil
}

// APIResponse represents a generic API response
type APIResponse struct {
	Data    any            `json:"data,omitempty"`
	Error   string         `json:"error,omitempty"`
	Message string         `json:"message,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}
