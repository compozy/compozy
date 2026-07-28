package model

import (
	"testing"
	"time"
)

func TestScopeNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		scope Scope
		want  Scope
	}{
		{name: "Should normalize global scope whitespace and case", scope: " Global ", want: ScopeGlobal},
		{name: "Should normalize workspace scope whitespace and case", scope: " WORKSPACE ", want: ScopeWorkspace},
		{name: "Should keep unsupported scope normalized", scope: " Tenant ", want: Scope("tenant")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.scope.Normalize(); got != tt.want {
				t.Fatalf("Normalize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestScopeValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		scope       Scope
		workspaceID string
		wantErr     string
	}{
		{name: "Should accept global scope without workspace", scope: ScopeGlobal},
		{name: "Should accept workspace scope with workspace id", scope: ScopeWorkspace, workspaceID: "ws-1"},
		{
			name:        "Should accept normalized workspace scope with workspace id",
			scope:       Scope(" WORKSPACE "),
			workspaceID: " ws-1 ",
		},
		{
			name:        "Should reject global scope with workspace id",
			scope:       ScopeGlobal,
			workspaceID: "ws-1",
			wantErr:     "global activation cannot include workspace id",
		},
		{
			name:    "Should reject workspace scope without workspace id",
			scope:   ScopeWorkspace,
			wantErr: "workspace activation requires workspace id",
		},
		{name: "Should reject empty scope", scope: Scope(" "), wantErr: "scope is required"},
		{name: "Should reject unsupported scope", scope: Scope("tenant"), wantErr: `unsupported scope "tenant"`},
		{
			name:    "Should reject unsupported normalized scope",
			scope:   Scope(" Tenant "),
			wantErr: `unsupported scope "tenant"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.scope.Validate(tt.workspaceID)
			assertError(t, err, tt.wantErr)
		})
	}
}

func TestActivationNormalize(t *testing.T) {
	t.Parallel()

	t.Run("Should canonicalize activation fields without changing timestamps", func(t *testing.T) {
		t.Parallel()

		createdAt := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
		updatedAt := time.Date(2026, 5, 6, 13, 0, 0, 0, time.UTC)
		got := Activation{
			ID:              " act_1 ",
			ExtensionName:   " marketing-team ",
			BundleName:      " marketing ",
			ProfileName:     " default ",
			Scope:           " WORKSPACE ",
			WorkspaceID:     " ws-1 ",
			SpecContentHash: " hash-1 ",
			CreatedAt:       createdAt,
			UpdatedAt:       updatedAt,
		}.Normalize()

		if got.ID != "act_1" {
			t.Fatalf("ID = %q, want %q", got.ID, "act_1")
		}
		if got.ExtensionName != "marketing-team" {
			t.Fatalf("ExtensionName = %q, want %q", got.ExtensionName, "marketing-team")
		}
		if got.BundleName != "marketing" {
			t.Fatalf("BundleName = %q, want %q", got.BundleName, "marketing")
		}
		if got.ProfileName != "default" {
			t.Fatalf("ProfileName = %q, want %q", got.ProfileName, "default")
		}
		if got.Scope != ScopeWorkspace {
			t.Fatalf("Scope = %q, want %q", got.Scope, ScopeWorkspace)
		}
		if got.WorkspaceID != "ws-1" {
			t.Fatalf("WorkspaceID = %q, want %q", got.WorkspaceID, "ws-1")
		}
		if got.SpecContentHash != "hash-1" {
			t.Fatalf("SpecContentHash = %q, want %q", got.SpecContentHash, "hash-1")
		}
		if !got.CreatedAt.Equal(createdAt) {
			t.Fatalf("CreatedAt = %s, want %s", got.CreatedAt, createdAt)
		}
		if !got.UpdatedAt.Equal(updatedAt) {
			t.Fatalf("UpdatedAt = %s, want %s", got.UpdatedAt, updatedAt)
		}
	})
}

func TestActivationValidate(t *testing.T) {
	t.Parallel()

	valid := Activation{
		ID:            "act_1",
		ExtensionName: "marketing-team",
		BundleName:    "marketing",
		ProfileName:   "default",
		Scope:         ScopeGlobal,
		CreatedAt:     time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 5, 6, 13, 0, 0, 0, time.UTC),
	}
	tests := []struct {
		name    string
		mutate  func(*Activation)
		wantErr string
	}{
		{name: "Should accept valid global activation"},
		{
			name: "Should accept valid workspace activation",
			mutate: func(next *Activation) {
				next.Scope = ScopeWorkspace
				next.WorkspaceID = "ws-1"
			},
		},
		{
			name: "Should reject missing activation id",
			mutate: func(next *Activation) {
				next.ID = " "
			},
			wantErr: "activation id is required",
		},
		{
			name: "Should reject missing extension name",
			mutate: func(next *Activation) {
				next.ExtensionName = " "
			},
			wantErr: "activation extension name is required",
		},
		{
			name: "Should reject missing bundle name",
			mutate: func(next *Activation) {
				next.BundleName = " "
			},
			wantErr: "activation bundle name is required",
		},
		{
			name: "Should reject missing profile name",
			mutate: func(next *Activation) {
				next.ProfileName = " "
			},
			wantErr: "activation profile name is required",
		},
		{
			name: "Should reject invalid scope binding",
			mutate: func(next *Activation) {
				next.WorkspaceID = "ws-1"
			},
			wantErr: "global activation cannot include workspace id",
		},
		{
			name: "Should reject empty activation scope",
			mutate: func(next *Activation) {
				next.Scope = " "
			},
			wantErr: "scope is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			activation := valid
			if tt.mutate != nil {
				tt.mutate(&activation)
			}
			err := activation.Validate()
			assertError(t, err, tt.wantErr)
		})
	}
}

func TestActivationValidated(t *testing.T) {
	t.Parallel()

	t.Run("Should return the canonical activation after validation", func(t *testing.T) {
		t.Parallel()

		got, err := Activation{
			ID:            " act_1 ",
			ExtensionName: " marketing-team ",
			BundleName:    " marketing ",
			ProfileName:   " default ",
			Scope:         " WORKSPACE ",
			WorkspaceID:   " ws-1 ",
		}.Validated()
		if err != nil {
			t.Fatalf("Validated() error = %v", err)
		}
		if got.ID != "act_1" {
			t.Fatalf("ID = %q, want %q", got.ID, "act_1")
		}
		if got.Scope != ScopeWorkspace {
			t.Fatalf("Scope = %q, want %q", got.Scope, ScopeWorkspace)
		}
		if got.WorkspaceID != "ws-1" {
			t.Fatalf("WorkspaceID = %q, want %q", got.WorkspaceID, "ws-1")
		}
	})

	t.Run("Should reject invalid activations without returning partial state", func(t *testing.T) {
		t.Parallel()

		got, err := Activation{
			ID:            " act_1 ",
			ExtensionName: " marketing-team ",
			BundleName:    " marketing ",
			ProfileName:   " default ",
			Scope:         " ",
		}.Validated()
		if err == nil {
			t.Fatal("Validated() error = nil, want validation error")
		}
		if got != (Activation{}) {
			t.Fatalf("Validated() activation = %+v, want zero value", got)
		}
	})
}

func TestInventoryItemValidate(t *testing.T) {
	t.Parallel()

	valid := InventoryItem{
		ActivationID:  "act_1",
		ResourceKind:  "agent",
		ResourceID:    "agt_1",
		ResourceName:  "marketer",
		RecordedAtUTC: time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
	}
	tests := []struct {
		name    string
		mutate  func(*InventoryItem)
		wantErr string
	}{
		{name: "Should accept valid inventory item"},
		{
			name: "Should reject missing activation id",
			mutate: func(next *InventoryItem) {
				next.ActivationID = " "
			},
			wantErr: "inventory activation id is required",
		},
		{
			name: "Should reject missing resource kind",
			mutate: func(next *InventoryItem) {
				next.ResourceKind = " "
			},
			wantErr: "inventory resource kind is required",
		},
		{
			name: "Should reject missing resource id",
			mutate: func(next *InventoryItem) {
				next.ResourceID = " "
			},
			wantErr: "inventory resource id is required",
		},
		{
			name: "Should reject missing resource name",
			mutate: func(next *InventoryItem) {
				next.ResourceName = " "
			},
			wantErr: "inventory resource name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			item := valid
			if tt.mutate != nil {
				tt.mutate(&item)
			}
			err := item.Validate()
			assertError(t, err, tt.wantErr)
		})
	}
}

func TestInventoryItemValidated(t *testing.T) {
	t.Parallel()

	t.Run("Should return the canonical inventory item after validation", func(t *testing.T) {
		t.Parallel()

		got, err := InventoryItem{
			ActivationID: " act_1 ",
			ResourceKind: " agent ",
			ResourceID:   " agt_1 ",
			ResourceName: " marketer ",
		}.Validated()
		if err != nil {
			t.Fatalf("Validated() error = %v", err)
		}
		if got.ActivationID != "act_1" {
			t.Fatalf("ActivationID = %q, want %q", got.ActivationID, "act_1")
		}
		if got.ResourceKind != "agent" {
			t.Fatalf("ResourceKind = %q, want %q", got.ResourceKind, "agent")
		}
		if got.ResourceID != "agt_1" {
			t.Fatalf("ResourceID = %q, want %q", got.ResourceID, "agt_1")
		}
		if got.ResourceName != "marketer" {
			t.Fatalf("ResourceName = %q, want %q", got.ResourceName, "marketer")
		}
	})
}

func assertError(t *testing.T, err error, want string) {
	t.Helper()

	if want == "" {
		if err != nil {
			t.Fatalf("error = %v, want nil", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("error = nil, want %q", want)
	}
	if err.Error() != "bundles: "+want {
		t.Fatalf("error = %q, want %q", err.Error(), "bundles: "+want)
	}
}
