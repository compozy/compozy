package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/gin-gonic/gin"
)

func TestParseSettingsOwner(t *testing.T) {
	t.Parallel()

	t.Run("Should infer profile scope from a non-default profile owner", func(t *testing.T) {
		t.Parallel()
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/settings?profile=marketing",
			http.NoBody,
		)

		scope, workspaceID, profileName, err := parseSettingsOwner(context)
		if err != nil {
			t.Fatalf("parseSettingsOwner() error = %v", err)
		}
		if scope != settingspkg.ScopeProfile || workspaceID != "" || profileName != "marketing" {
			t.Fatalf(
				"parseSettingsOwner() = %q/%q/%q, want profile/empty/marketing",
				scope,
				workspaceID,
				profileName,
			)
		}
	})

	t.Run("Should reject incomplete profile scopes", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name  string
			query string
		}{
			{name: "missing profile", query: "scope=profile"},
			{name: "default profile", query: "scope=profile&profile=default"},
			{name: "invalid profile name", query: "scope=profile&profile=bad%2Fname"},
		}
		for _, test := range tests {
			t.Run("Should reject "+test.name, func(t *testing.T) {
				t.Parallel()
				context, _ := gin.CreateTestContext(httptest.NewRecorder())
				context.Request = httptest.NewRequestWithContext(
					t.Context(),
					http.MethodGet,
					"/settings?"+test.query,
					http.NoBody,
				)

				_, _, _, err := parseSettingsOwner(context)
				if err == nil {
					t.Fatalf("parseSettingsOwner(%q) error = nil, want validation error", test.query)
				}
			})
		}
	})
}
