package profile

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	namePattern  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	colorPattern = regexp.MustCompile(`^#[0-9a-f]{6}$`)
	reserved     = map[string]struct{}{"default": {}, "all": {}, "global": {}}
)

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !namePattern.MatchString(name) {
		return "", domainError(
			"profile_name_invalid",
			fmt.Sprintf("profile name %q must match %s", name, namePattern.String()),
			"use a lowercase name such as marketing or dev-tools",
			ErrNameInvalid,
		)
	}
	if _, found := reserved[name]; found {
		return "", domainError(
			"profile_name_reserved",
			fmt.Sprintf("profile name %q is reserved", name),
			"choose a name other than default, all, or global",
			ErrNameReserved,
		)
	}
	return name, nil
}

// NormalizeName validates and returns the canonical profile name used by
// manifests and other boundaries that bind to profiles by name.
func NormalizeName(name string) (string, error) {
	return normalizeName(name)
}

// NormalizeIdentity validates and defaults the shared profile identity shape.
func NormalizeIdentity(color, icon, emoji string) (string, string, string, error) {
	return normalizeIdentity(color, icon, emoji)
}

func normalizeIdentity(color, icon, emoji string) (string, string, string, error) {
	color = strings.ToLower(strings.TrimSpace(color))
	icon = strings.TrimSpace(icon)
	emoji = strings.TrimSpace(emoji)
	if color == "" {
		color = "#8e8eb5"
	}
	if !colorPattern.MatchString(color) {
		return "", "", "", domainError(
			"profile_identity_invalid", "profile color must use #rrggbb", "choose a six-digit lowercase hex color", ErrInvalidInput,
		)
	}
	if icon == "" && emoji == "" {
		icon = "circle"
	}
	if (icon == "") == (emoji == "") {
		return "", "", "", domainError(
			"profile_identity_invalid", "exactly one profile icon or emoji is required", "set either icon or emoji", ErrInvalidInput,
		)
	}
	return color, icon, emoji, nil
}

func (l Lens) Validate() error {
	workspaceID := strings.TrimSpace(l.WorkspaceID)
	switch l.Kind {
	case SelectionLensGlobal:
		if workspaceID != "" {
			return fmt.Errorf("%w: global lens forbids workspace id", ErrInvalidInput)
		}
	case SelectionLensWorkspace:
		if workspaceID == "" {
			return fmt.Errorf("%w: workspace lens requires workspace id", ErrInvalidInput)
		}
	default:
		return fmt.Errorf("%w: unsupported selection lens %q", ErrInvalidInput, l.Kind)
	}
	return nil
}

func (r RepoChoice) Validate() error {
	forms := 0
	if r.All {
		forms++
	}
	if r.None {
		forms++
	}
	if len(r.WorkspaceIDs) > 0 {
		forms++
		seen := make(map[string]struct{}, len(r.WorkspaceIDs))
		for _, workspaceID := range r.WorkspaceIDs {
			workspaceID = strings.TrimSpace(workspaceID)
			if workspaceID == "" {
				return fmt.Errorf("%w: repository workspace id is required", ErrInvalidInput)
			}
			if _, duplicate := seen[workspaceID]; duplicate {
				return fmt.Errorf("%w: duplicate repository workspace id %q", ErrInvalidInput, workspaceID)
			}
			seen[workspaceID] = struct{}{}
		}
	}
	if forms != 1 {
		return fmt.Errorf("%w: repository choice requires exactly one of all, none, or workspace ids", ErrInvalidInput)
	}
	return nil
}
