package logger

import (
	"github.com/charmbracelet/lipgloss"
	charmlog "github.com/charmbracelet/log"
)

// getDefaultStyles returns a charmlog.Styles struct with custom Lipgloss styles
// using a Tailwind CSS inspired 500-series color palette for foreground colors.
func getDefaultStyles() *charmlog.Styles {
	s := charmlog.DefaultStyles()

	// Timestamp Style (Tailwind: slate-400)
	s.Timestamp = lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8"))

	// Level Styles (using 500-series Tailwind colors for foreground)
	s.Levels[charmlog.DebugLevel] = lipgloss.NewStyle().
		SetString("DEBUG").
		Padding(0, 1, 0, 1).                  // User's preference
		Foreground(lipgloss.Color("#10b981")) // Tailwind: emerald-500

	s.Levels[charmlog.InfoLevel] = lipgloss.NewStyle().
		SetString("INFO ").
		Padding(0, 1, 0, 1).                  // User's preference
		Foreground(lipgloss.Color("#6366f1")) // Tailwind: indigo-500

	s.Levels[charmlog.WarnLevel] = lipgloss.NewStyle().
		SetString("WARN ").
		Padding(0, 1, 0, 1).                  // User's preference
		Foreground(lipgloss.Color("#f59e0b")) // Tailwind: amber-500

	s.Levels[charmlog.ErrorLevel] = lipgloss.NewStyle().
		SetString("ERROR!!").
		Padding(0, 1, 0, 1).                  // User's preference
		Foreground(lipgloss.Color("#ec4899")) // Tailwind: pink-500

	s.Levels[charmlog.FatalLevel] = lipgloss.NewStyle().
		SetString("FATAL").
		Padding(0, 1, 0, 1).                  // User's preference
		Foreground(lipgloss.Color("#ef4444")) // Tailwind: red-500

	// Key-Value Styles
	// Specific style for "err" key
	s.Keys["err"] = lipgloss.NewStyle().Foreground(lipgloss.Color("#ec4899"))              // Tailwind: pink-500
	s.Values["err"] = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ec4899")) // Tailwind: pink-500

	// Default key-value styling for other keys
	s.Key = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))   // Tailwind: gray-500 (Medium Gray)
	s.Value = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")) // White

	// Caller Style (source file location) - Tailwind: slate-500
	s.Caller = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("#64748b"))

	return s
}
