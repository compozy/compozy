package extensionpkg

import (
	"errors"
	"slices"
	"testing"
)

func TestCapabilityCheckerModelHostAPIMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		actions    []string
		security   []string
		method     string
		wantError  bool
		wantNeeded []string
	}{
		{
			name:     "Should allow models list with read grant",
			actions:  []string{"models/list"},
			security: []string{"model.read"},
			method:   "models/list",
		},
		{
			name:     "Should allow models status with read grant",
			actions:  []string{"models/status"},
			security: []string{"model.read"},
			method:   "models/status",
		},
		{
			name:     "Should allow models refresh with write grant",
			actions:  []string{"models/refresh"},
			security: []string{"model.write"},
			method:   "models/refresh",
		},
		{
			name:       "Should reject models refresh without write grant",
			actions:    []string{"models/refresh"},
			security:   []string{"model.read"},
			method:     "models/refresh",
			wantError:  true,
			wantNeeded: []string{"model.write"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := newTestCapabilityChecker("ext", SourceUser, tt.actions, tt.security)
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
			Actions: ActionsConfig{
				Requires: []string{"models/list", "models/refresh", "models/status", "sessions/list"},
			},
			Security: SecurityConfig{
				Capabilities: []string{"model.read", "model.write", "session.read"},
			},
		})

		grant := checker.Grant("ext")
		if slices.Contains(grant.Actions, "models/list") ||
			slices.Contains(grant.Actions, "models/refresh") ||
			slices.Contains(grant.Actions, "models/status") {
			t.Fatalf("Grant.Actions = %v, want marketplace model actions denied by source tier ceiling", grant.Actions)
		}
		if slices.Contains(grant.Security, "model.read") || slices.Contains(grant.Security, "model.write") {
			t.Fatalf(
				"Grant.Security = %v, want marketplace model security denied by source tier ceiling",
				grant.Security,
			)
		}
		if !slices.Equal(grant.Actions, []string{"sessions/list"}) {
			t.Fatalf("Grant.Actions = %v, want [sessions/list]", grant.Actions)
		}
		if !slices.Equal(grant.Security, []string{"session.read"}) {
			t.Fatalf("Grant.Security = %v, want [session.read]", grant.Security)
		}
	})
}
