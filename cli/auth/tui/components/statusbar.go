package components

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/compozy/compozy/cli/auth/tui/styles"
)

// StatusBarComponent displays status information and progress
type StatusBarComponent struct {
	Width    int
	Left     string
	Center   string
	Right    string
	Loading  bool
	Progress float64 // 0.0 to 1.0
}

// NewStatusBar creates a new status bar component
func NewStatusBar(width int) StatusBarComponent {
	return StatusBarComponent{
		Width: width,
	}
}

// SetLeft sets the left content
func (s StatusBarComponent) SetLeft(content string) StatusBarComponent {
	s.Left = content
	return s
}

// SetCenter sets the center content
func (s StatusBarComponent) SetCenter(content string) StatusBarComponent {
	s.Center = content
	return s
}

// SetRight sets the right content
func (s StatusBarComponent) SetRight(content string) StatusBarComponent {
	s.Right = content
	return s
}

// SetLoading sets the loading state
func (s StatusBarComponent) SetLoading(loading bool) StatusBarComponent {
	s.Loading = loading
	return s
}

// SetProgress sets the progress value (0.0 to 1.0)
func (s StatusBarComponent) SetProgress(progress float64) StatusBarComponent {
	s.Progress = progress
	return s
}

// Update handles status bar updates
func (s StatusBarComponent) Update(msg tea.Msg) (StatusBarComponent, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.Width = msg.Width
	case StatusUpdateMsg:
		s.Left = msg.Left
		s.Center = msg.Center
		s.Right = msg.Right
	case LoadingMsg:
		s.Loading = msg.Loading
	case ProgressMsg:
		s.Progress = msg.Progress
	}
	return s, nil
}

// View renders the status bar
func (s StatusBarComponent) View() string {
	left := s.Left
	center := s.Center
	right := s.Right

	// Add loading indicator if loading
	if s.Loading {
		spinner := s.getSpinner()
		if left != "" {
			left = spinner + " " + left
		} else {
			left = spinner
		}
	}

	// Add progress if set
	if s.Progress > 0 {
		progressBar := s.renderProgress()
		if center != "" {
			center = center + " " + progressBar
		} else {
			center = progressBar
		}
	}

	// Add timestamp to right if empty
	if right == "" {
		right = time.Now().Format("15:04:05")
	}

	return styles.RenderStatusBar(s.Width, left, center, right)
}

// getSpinner returns a spinning character based on current time
func (s StatusBarComponent) getSpinner() string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frame := int(time.Now().UnixMilli()/100) % len(frames)
	return styles.SpinnerStyle.Render(frames[frame])
}

// renderProgress renders a progress bar
func (s StatusBarComponent) renderProgress() string {
	if s.Progress <= 0 {
		return ""
	}

	width := 20
	filled := int(s.Progress * float64(width))
	progress := fmt.Sprintf("[%s%s] %d%%",
		styles.ActivePageStyle.Render(string(make([]rune, filled))),
		string(make([]rune, width-filled)),
		int(s.Progress*100))

	return progress
}

// Height returns the height of the status bar (always 1)
func (s StatusBarComponent) Height() int {
	return 1
}

// StatusUpdateMsg updates status bar content
type StatusUpdateMsg struct {
	Left   string
	Center string
	Right  string
}

// LoadingMsg sets loading state
type LoadingMsg struct {
	Loading bool
}

// ProgressMsg sets progress value
type ProgressMsg struct {
	Progress float64
}

// NewStatusUpdateMsg creates a status update message
func NewStatusUpdateMsg(left, center, right string) StatusUpdateMsg {
	return StatusUpdateMsg{
		Left:   left,
		Center: center,
		Right:  right,
	}
}

// NewLoadingMsg creates a loading message
func NewLoadingMsg(loading bool) LoadingMsg {
	return LoadingMsg{Loading: loading}
}

// NewProgressMsg creates a progress message
func NewProgressMsg(progress float64) ProgressMsg {
	return ProgressMsg{Progress: progress}
}
