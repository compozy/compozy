package extensionpkg

import (
	"errors"
	"slices"
	"testing"
)

func TestCapabilityCheckerModelHostAPIMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		permissions []string
		method      string
		wantError   bool
		wantNeeded  []string
	}{
		{
			name:        "Should allow models list with read grant",
			permissions: []string{"models/list"},
			method:      "models/list",
		},
		{
			name:        "Should allow models status with read grant",
			permissions: []string{"models/status"},
			method:      "models/status",
		},
		{
			name:        "Should allow models refresh with write grant",
			permissions: []string{"models/refresh"},
			method:      "models/refresh",
		},
		{
			name:        "Should reject an undeclared models refresh permission",
			permissions: []string{"models/list"},
			method:      "models/refresh",
			wantError:   true,
			wantNeeded:  []string{"models/refresh"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := newTestCapabilityChecker("ext", SourceUser, tt.permissions)
			err := checker.CheckHostAPI("ext", tt.method)
			if !tt.wantError {
				if err != nil {
					t.Fatalf("CheckHostAPI(%q) error = %v, want nil", tt.method, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("CheckHostAPI(%q) error = nil, want capability denied", tt.method)
			}
			var denied *ErrCapabilityDenied
			if !errors.As(err, &denied) {
				t.Fatalf("CheckHostAPI(%q) error = %T, want *ErrCapabilityDenied", tt.method, err)
			}
			if !slices.Equal(denied.Data.Required, tt.wantNeeded) {
				t.Fatalf("Data.Required = %v, want %v", denied.Data.Required, tt.wantNeeded)
			}
		})
	}
}

func TestCapabilityCheckerMarketplaceModelCeilings(t *testing.T) {
	t.Parallel()

	t.Run("Should deny marketplace model Host API methods", func(t *testing.T) {
		t.Parallel()

		checker := newTestCapabilityChecker(
			"ext",
			SourceMarketplace,
			[]string{"models/list", "models/refresh", "models/status"},
			[]string{"model.read", "model.write"},
		)
		for _, method := range []string{"models/list", "models/refresh", "models/status"} {
			t.Run("Should deny "+method+" for marketplace sources", func(t *testing.T) {
				t.Parallel()

				err := checker.CheckHostAPI("ext", method)
				if err == nil {
					t.Fatalf("CheckHostAPI(%q) error = nil, want capability denied", method)
				}
				var denied *ErrCapabilityDenied
				if !errors.As(err, &denied) {
					t.Fatalf("CheckHostAPI(%q) error = %T, want *ErrCapabilityDenied", method, err)
				}
			})
		}
	})

	t.Run("Should remove marketplace model grants from effective grant", func(t *testing.T) {
		t.Parallel()

		checker := &CapabilityChecker{}
		checker.Register("ext", SourceMarketplace, &Manifest{
			Permissions: PermissionsConfig{
				Requires: []string{"models/list", "models/refresh", "models/status", "sessions/list"},
			},
		})

		grant := checker.Grant("ext")
		if slices.Contains(grant.Permissions, "models/list") ||
			slices.Contains(grant.Permissions, "models/refresh") ||
			slices.Contains(grant.Permissions, "models/status") {
			t.Fatalf(
				"Grant.Permissions = %v, want marketplace model permissions denied by source tier ceiling",
				grant.Permissions,
			)
		}
		if slices.Contains(grant.Security, "model.read") || slices.Contains(grant.Security, "model.write") {
			t.Fatalf(
				"Grant.Security = %v, want derived marketplace model consent denied by source tier ceiling",
				grant.Security,
			)
		}
		if !slices.Equal(grant.Permissions, []string{"sessions/list"}) {
			t.Fatalf("Grant.Permissions = %v, want [sessions/list]", grant.Permissions)
		}
		if !slices.Equal(grant.Security, []string{"session.read"}) {
			t.Fatalf("Grant.Security = %v, want [session.read]", grant.Security)
		}
	})
}
