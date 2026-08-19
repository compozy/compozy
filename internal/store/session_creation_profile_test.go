package store

import (
	"strings"
	"testing"

	speedpkg "github.com/compozy/compozy/internal/speed"
)

func TestSessionCreationProfileShouldBindSpeedToVersionedIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should persist normal speed in the canonical version three profile", func(t *testing.T) {
		t.Parallel()

		profile := validSessionCreationProfile()
		payload, err := profile.CanonicalJSON()
		if err != nil {
			t.Fatalf("CanonicalJSON() error = %v", err)
		}
		encoded := string(payload)
		if !strings.Contains(encoded, `"version":3`) || !strings.Contains(encoded, `"speed":"normal"`) {
			t.Fatalf("CanonicalJSON() = %s, want version 3 with normal speed", encoded)
		}
	})

	t.Run("Should change the immutable profile reference when speed changes", func(t *testing.T) {
		t.Parallel()

		normal := validSessionCreationProfile()
		fast := normal
		fast.Speed = speedpkg.SpeedFast
		normalRef, err := normal.Ref()
		if err != nil {
			t.Fatalf("normal Ref() error = %v", err)
		}
		fastRef, err := fast.Ref()
		if err != nil {
			t.Fatalf("fast Ref() error = %v", err)
		}
		if normalRef == fastRef {
			t.Fatalf("profile refs = %q, want speed-specific identities", normalRef)
		}
	})

	t.Run("Should reject version two and unsupported speeds", func(t *testing.T) {
		t.Parallel()

		versionTwo := validSessionCreationProfile()
		versionTwo.Version = 2
		if err := versionTwo.Validate(); err == nil ||
			!strings.Contains(err.Error(), "unsupported session creation profile version 2") {
			t.Fatalf("Validate(version 2) error = %v, want hard-cut version rejection", err)
		}

		invalidSpeed := validSessionCreationProfile()
		invalidSpeed.Speed = speedpkg.Speed("turbo")
		if err := invalidSpeed.Validate(); err == nil || !strings.Contains(err.Error(), "expected normal or fast") {
			t.Fatalf("Validate(turbo) error = %v, want closed speed rejection", err)
		}
	})
}

func validSessionCreationProfile() SessionCreationProfile {
	return SessionCreationProfile{
		Version:     SessionCreationProfileVersion,
		AgentName:   "worker",
		Provider:    "codex",
		WorkspaceID: "workspace:test",
		CWD:         "/workspace",
		SandboxMode: SessionCreationSandboxNone,
		Permissions: "approve-all",
	}
}
