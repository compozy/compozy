package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/compozy/compozy/cli/auth/tui/styles"
)

// ErrorComponent displays error messages with retry options
type ErrorComponent struct {
	Error       error
	Retryable   bool
	ShowDetails bool
}

// NewErrorComponent creates a new error component
func NewErrorComponent(err error, retryable bool) ErrorComponent {
	return ErrorComponent{
		Error:     err,
		Retryable: retryable,
	}
}

// Update handles input for the error component
func (c ErrorComponent) Update(msg tea.Msg) (ErrorComponent, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "d" {
			// Toggle details
			c.ShowDetails = !c.ShowDetails
		}
	}
	return c, nil
}

// View renders the error component
func (c ErrorComponent) View() string {
	if c.Error == nil {
		return ""
	}

	var content string

	// Error message
	content += styles.RenderError(c.Error.Error()) + "\n"

	// Show details if requested
	if c.ShowDetails {
		content += styles.ErrorBoxStyle.Render(c.Error.Error()) + "\n"
	}

	// Help text
	var help [][2]string
	if c.Retryable {
		help = append(help, [2]string{"r", "retry"})
	}
	help = append(help, [2]string{"d", "toggle details"}, [2]string{"q", "quit"})

	content += "\n" + styles.RenderHelp(help)

	return content
}

// Height returns the height of the error component
func (c ErrorComponent) Height() int {
	if c.Error == nil {
		return 0
	}

	height := 2 // Error message + spacing
	if c.ShowDetails {
		height += 3 // Error box with padding
	}
	height += 2 // Help text

	return height
}

// ErrorMsg represents an error message
type ErrorMsg struct {
	Err       error
	Retryable bool
}

// NewErrorMsg creates a new error message
func NewErrorMsg(err error, retryable bool) ErrorMsg {
	return ErrorMsg{
		Err:       err,
		Retryable: retryable,
	}
}

// RetryMsg represents a retry command
type RetryMsg struct{}

// ClearErrorMsg clears the current error
type ClearErrorMsg struct{}
