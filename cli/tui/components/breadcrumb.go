package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/compozy/cli/tui/styles"
)

// BreadcrumbItem represents a single breadcrumb item
type BreadcrumbItem struct {
	Label  string
	Active bool
}

// Breadcrumb provides navigation breadcrumb for deep workflows
type Breadcrumb struct {
	Width int
	Items []BreadcrumbItem
}

// NewBreadcrumb creates a new breadcrumb component
func NewBreadcrumb() Breadcrumb {
	return Breadcrumb{
		Items: make([]BreadcrumbItem, 0),
	}
}

// SetWidth sets the breadcrumb width
func (b *Breadcrumb) SetWidth(width int) {
	b.Width = width
}

// SetItems sets the breadcrumb items
func (b *Breadcrumb) SetItems(items []BreadcrumbItem) {
	b.Items = items
}

// AddItem adds a breadcrumb item
func (b *Breadcrumb) AddItem(label string, active bool) {
	for i := range b.Items {
		b.Items[i].Active = false
	}
	b.Items = append(b.Items, BreadcrumbItem{
		Label:  label,
		Active: active,
	})
}

// PopItem removes the last breadcrumb item
func (b *Breadcrumb) PopItem() {
	if len(b.Items) > 0 {
		b.Items = b.Items[:len(b.Items)-1]
		if len(b.Items) > 0 {
			b.Items[len(b.Items)-1].Active = true
		}
	}
}

// Clear removes all breadcrumb items
func (b *Breadcrumb) Clear() {
	b.Items = make([]BreadcrumbItem, 0)
}

// View renders the breadcrumb
func (b *Breadcrumb) View() string {
	if len(b.Items) == 0 {
		return ""
	}
	var parts []string
	for i, item := range b.Items {
		var rendered string

		if item.Active {
			rendered = styles.BreadcrumbActiveStyle.Render(item.Label)
		} else {
			rendered = styles.BreadcrumbStyle.Render(item.Label)
		}

		parts = append(parts, rendered)

		if i < len(b.Items)-1 {
			separator := styles.BreadcrumbStyle.Render(" → ")
			parts = append(parts, separator)
		}
	}
	breadcrumb := strings.Join(parts, "")
	if b.Width > 0 && lipgloss.Width(breadcrumb) > b.Width {
		maxWidth := b.Width - 3 // Leave space for "..."
		for lipgloss.Width(breadcrumb) > maxWidth && len(parts) > 1 {
			if len(parts) >= 3 {
				parts = parts[2:] // Remove item and separator
			} else {
				break
			}
			breadcrumb = strings.Join(parts, "")
		}
		if len(parts) > 0 {
			breadcrumb = "..." + breadcrumb
		}
	}
	return breadcrumb
}

// GetHeight returns the breadcrumb height (always 1)
func (b *Breadcrumb) GetHeight() int {
	if len(b.Items) == 0 {
		return 0
	}
	return 1
}

// Helper functions for common breadcrumb patterns

// setPath is a helper to set a breadcrumb path with optional last item
func (b *Breadcrumb) setPath(items []string, lastItem string) {
	b.Clear()
	for _, item := range items {
		b.AddItem(item, false)
	}
	if lastItem != "" {
		b.AddItem(lastItem, true)
	} else if len(b.Items) > 0 {
		b.Items[len(b.Items)-1].Active = true
	}
}

// SetAuthPath sets breadcrumb for auth commands
func (b *Breadcrumb) SetAuthPath(subcommand string) {
	b.setPath([]string{"Compozy", "Auth"}, subcommand)
}

// SetKeyPath sets breadcrumb for key management
func (b *Breadcrumb) SetKeyPath(action string) {
	b.setPath([]string{"Compozy", "Auth", "Keys"}, action)
}

// SetUserPath sets breadcrumb for user management
func (b *Breadcrumb) SetUserPath(action string) {
	b.setPath([]string{"Compozy", "Auth", "Users"}, action)
}
