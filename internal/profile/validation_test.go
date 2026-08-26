package profile

import (
	"errors"
	"testing"
)

func TestNormalizeIdentitySymbols(t *testing.T) {
	t.Parallel()

	t.Run("Should default to a renderable catalog icon when no symbol is given", func(t *testing.T) {
		t.Parallel()

		color, icon, emoji, err := NormalizeIdentity("", "", "")
		if err != nil {
			t.Fatalf("NormalizeIdentity() error = %v", err)
		}
		if color != "#8e8eb5" || emoji != "" {
			t.Fatalf("NormalizeIdentity() = (%q, %q, %q), want default color and no emoji", color, icon, emoji)
		}
		if !isCatalogIcon(icon) {
			t.Fatalf("default icon %q is not in the embedded Lucide catalog", icon)
		}
	})

	t.Run("Should accept any catalog icon and a multi-codepoint emoji", func(t *testing.T) {
		t.Parallel()

		if _, icon, _, err := NormalizeIdentity("#123abc", "banana", ""); err != nil || icon != "banana" {
			t.Fatalf("NormalizeIdentity(banana) = (%q, %v), want banana accepted", icon, err)
		}
		if _, _, emoji, err := NormalizeIdentity("", "", "👨‍👩‍👧‍👦"); err != nil || emoji != "👨‍👩‍👧‍👦" {
			t.Fatalf("NormalizeIdentity(family emoji) = (%q, %v), want emoji accepted", emoji, err)
		}
	})

	t.Run("Should reject emoji containing spaces or control characters", func(t *testing.T) {
		t.Parallel()

		for _, emoji := range []string{"a b", "x\ny", "🦊\a🦊"} {
			if _, _, _, err := NormalizeIdentity("", "", emoji); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("NormalizeIdentity(%q) error = %v, want ErrInvalidInput", emoji, err)
			}
		}
	})
}

func TestSelectionAndRepositoryChoiceValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should validate selection lenses", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			lens  Lens
			valid bool
		}{
			{name: "Should accept global lens", lens: Lens{Kind: SelectionLensGlobal}, valid: true},
			{
				name:  "Should accept workspace lens",
				lens:  Lens{Kind: SelectionLensWorkspace, WorkspaceID: "workspace-1"},
				valid: true,
			},
			{
				name: "Should reject global lens with workspace",
				lens: Lens{Kind: SelectionLensGlobal, WorkspaceID: "workspace-1"},
			},
			{name: "Should reject workspace lens without workspace", lens: Lens{Kind: SelectionLensWorkspace}},
			{name: "Should reject unknown lens", lens: Lens{Kind: SelectionLens("unknown")}},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				err := test.lens.Validate()
				if test.valid && err != nil {
					t.Fatalf("Lens.Validate() error = %v, want nil", err)
				}
				if !test.valid && !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("Lens.Validate() error = %v, want ErrInvalidInput", err)
				}
			})
		}
	})

	t.Run("Should validate repository choices", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			choice RepoChoice
			valid  bool
		}{
			{name: "Should accept all", choice: RepoChoice{All: true}, valid: true},
			{name: "Should accept none", choice: RepoChoice{None: true}, valid: true},
			{
				name:   "Should accept workspace ids",
				choice: RepoChoice{WorkspaceIDs: []string{"workspace-1", "workspace-2"}},
				valid:  true,
			},
			{name: "Should reject empty choice", choice: RepoChoice{}},
			{name: "Should reject multiple choices", choice: RepoChoice{All: true, None: true}},
			{name: "Should reject blank workspace id", choice: RepoChoice{WorkspaceIDs: []string{""}}},
			{
				name:   "Should reject duplicate workspace id",
				choice: RepoChoice{WorkspaceIDs: []string{"workspace-1", "workspace-1"}},
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				err := test.choice.Validate()
				if test.valid && err != nil {
					t.Fatalf("RepoChoice.Validate() error = %v, want nil", err)
				}
				if !test.valid && !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("RepoChoice.Validate() error = %v, want ErrInvalidInput", err)
				}
			})
		}
	})
}
