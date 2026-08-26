package profile

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

var (
	namePattern  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	colorPattern = regexp.MustCompile(`^#[0-9a-f]{6}$`)
	reserved     = map[string]struct{}{string(ResolutionSourceDefault): {}, "all": {}, "global": {}}
)

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if !namePattern.MatchString(name) {
		return "", domainError(
			"profile_name_invalid",
			fmt.Sprintf(
				"profile name %q must start with a lowercase letter and contain only lowercase letters, "+
					"numbers, and hyphens (up to 32 characters)",
				name,
			),
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

// NormalizeName trims and validates a profile name, returning the canonical
// grammar-safe form (lowercase, 32 characters or fewer) for manifests and
// other boundaries that bind to profiles by name.
func NormalizeName(name string) (string, error) {
	return normalizeName(name)
}

// NormalizeIdentity trims identity fields, lowercases the color, applies the
// default color and icon, and validates the shared profile identity shape.
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
			"profile_identity_invalid",
			"profile color must use #rrggbb",
			"choose a six-digit lowercase hex color",
			ErrInvalidInput,
		)
	}
	if icon == "" && emoji == "" {
		icon = "circle"
	}
	if (icon == "") == (emoji == "") {
		return "", "", "", domainError(
			"profile_identity_invalid",
			"exactly one profile icon or emoji is required",
			"set either icon or emoji",
			ErrInvalidInput,
		)
	}
	if icon != "" && !isCatalogIcon(icon) {
		return "", "", "", domainError(
			"profile_identity_invalid",
			fmt.Sprintf("profile icon %q is not in the Lucide icon catalog", icon),
			"use a Lucide icon name such as rocket or user-round",
			ErrInvalidInput,
		)
	}
	if emoji != "" && !isRenderableEmoji(emoji) {
		return "", "", "", domainError(
			"profile_identity_invalid",
			"profile emoji must be a single emoji grapheme of 64 bytes or fewer",
			"pick one emoji",
			ErrInvalidInput,
		)
	}
	return color, icon, emoji, nil
}

// maxEmojiBytes bounds one emoji grapheme; the longest RGI ZWJ sequences stay far under it.
const maxEmojiBytes = 64

func isRenderableEmoji(emoji string) bool {
	if len(emoji) > maxEmojiBytes || !utf8.ValidString(emoji) {
		return false
	}
	if strings.ContainsFunc(emoji, unicode.IsControl) {
		return false
	}
	return uniseg.GraphemeClusterCount(emoji) == 1
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
