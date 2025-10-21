package components

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/tui/styles"
)

const escKey = "esc"

// TutorialStep represents a single tutorial step
type TutorialStep struct {
	Title       string
	Description string
	Action      string
	KeyBinding  string
}

// Tutorial provides an interactive tutorial for first-time users
type Tutorial struct {
	Width    int
	Height   int
	Visible  bool
	Steps    []TutorialStep
	Current  int
	Complete bool
}

// NewTutorial creates a new tutorial component
func NewTutorial() Tutorial {
	return Tutorial{
		Visible: false,
		Current: 0,
		Steps:   defaultTutorialSteps(),
	}
}

// defaultTutorialSteps returns the default tutorial steps
func defaultTutorialSteps() []TutorialStep {
	return []TutorialStep{
		tutorialWelcomeStep(),
		tutorialHelpStep(),
		tutorialPaletteStep(),
		tutorialNavigationStep(),
		tutorialKeyManagementStep(),
		tutorialUserManagementStep(),
		tutorialCompletionStep(),
	}
}

func tutorialWelcomeStep() TutorialStep {
	return TutorialStep{
		Title: "Welcome to Compozy Auth",
		Description: "This tutorial will guide you through the basic features of the Compozy Auth CLI. " +
			"Use arrow keys to navigate and Enter to continue.",
		Action:     "Press Enter to continue",
		KeyBinding: "enter",
	}
}

func tutorialHelpStep() TutorialStep {
	return TutorialStep{
		Title: "Getting Help",
		Description: "Press '?' at any time to see context-sensitive help for the current screen. " +
			"This shows all available keyboard shortcuts and actions.",
		Action:     "Try pressing '?' now",
		KeyBinding: "?",
	}
}

func tutorialPaletteStep() TutorialStep {
	return TutorialStep{
		Title: "Command Palette",
		Description: "Press Ctrl+K to open the command palette. " +
			"This provides quick access to all available actions and commands.",
		Action:     "Try pressing Ctrl+K",
		KeyBinding: "ctrl+k",
	}
}

func tutorialNavigationStep() TutorialStep {
	return TutorialStep{
		Title: "Navigation",
		Description: "Use arrow keys or Vim-style keys (h/j/k/l) to navigate through lists and forms. " +
			"Tab moves between form fields.",
		Action:     "Practice with arrow keys",
		KeyBinding: "arrows",
	}
}

func tutorialKeyManagementStep() TutorialStep {
	return TutorialStep{
		Title: "Key Management",
		Description: "Generate API keys with 'compozy auth generate', list them with 'compozy auth list', " +
			"and revoke them when no longer needed.",
		Action:     "Remember these commands",
		KeyBinding: "enter",
	}
}

func tutorialUserManagementStep() TutorialStep {
	return TutorialStep{
		Title: "User Management",
		Description: "Admin users can create, list, update, and delete user accounts. " +
			"Regular users can only view their own information.",
		Action:     "Understand user roles",
		KeyBinding: "enter",
	}
}

func tutorialCompletionStep() TutorialStep {
	return TutorialStep{
		Title: "Tutorial Complete",
		Description: "You've completed the tutorial! You can restart it anytime from the command palette. " +
			"Press Esc to close this tutorial.",
		Action:     "Press Esc to finish",
		KeyBinding: escKey,
	}
}

// SetSize sets the tutorial size
func (t *Tutorial) SetSize(width, height int) {
	t.Width = width
	t.Height = height
}

// Start starts the tutorial
func (t *Tutorial) Start() {
	t.Visible = true
	t.Current = 0
	t.Complete = false
}

// Hide hides the tutorial
func (t *Tutorial) Hide() {
	t.Visible = false
}

// Next advances to the next step
func (t *Tutorial) Next() {
	if t.Current < len(t.Steps)-1 {
		t.Current++
	} else {
		t.Complete = true
	}
}

// Previous goes to the previous step
func (t *Tutorial) Previous() {
	if t.Current > 0 {
		t.Current--
	}
}

// Update handles tutorial updates
func (t *Tutorial) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		if !t.Visible {
			return nil
		}

		switch msg.String() {
		case escKey:
			t.Hide()
			return nil

		case "enter", "space":
			if t.Complete {
				t.Hide()
			} else {
				t.Next()
			}

		case "left", "h":
			t.Previous()

		case "right", "l":
			t.Next()

		case "q":
			t.Hide()
			return nil
		}

	case StartTutorialMsg:
		t.Start()

	case HideTutorialMsg:
		t.Hide()
	}
	return nil
}

// View renders the tutorial
func (t *Tutorial) View() string {
	if !t.Visible {
		return ""
	}
	step := t.Steps[t.Current]
	content := t.renderStep(step)
	dialog := styles.DialogStyle.
		Width(t.Width - 8).
		Render(content)
	return lipgloss.Place(t.Width, t.Height, lipgloss.Center, lipgloss.Center, dialog)
}

// renderStep renders a tutorial step
func (t *Tutorial) renderStep(step TutorialStep) string {
	var content string
	progress := styles.HelpStyle.Render(fmt.Sprintf("Step %d of %d", t.Current+1, len(t.Steps)))
	content += progress + "\n\n"
	content += styles.RenderTitle(step.Title) + "\n\n"
	content += step.Description + "\n\n"
	content += styles.HelpKeyStyle.Render(step.Action) + "\n\n"
	nav := ""
	if t.Current > 0 {
		nav += styles.HelpStyle.Render("← previous")
	}
	if t.Current > 0 && t.Current < len(t.Steps)-1 {
		nav += " • "
	}
	if t.Current < len(t.Steps)-1 {
		nav += styles.HelpStyle.Render("next →")
	}
	if nav != "" {
		content += nav + "\n"
	}
	content += styles.HelpStyle.Render("Press q or esc to close tutorial")
	return content
}

// Tutorial Messages

// StartTutorialMsg starts the tutorial
type StartTutorialMsg struct{}

// HideTutorialMsg hides the tutorial
type HideTutorialMsg struct{}
