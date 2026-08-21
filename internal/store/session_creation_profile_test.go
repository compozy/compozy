package store

import (
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/network/participation"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

func TestSessionCreationProfileShouldBindRuntimePolicyToVersionedIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should persist normal speed and profile id in the canonical version four profile", func(t *testing.T) {
		t.Parallel()

		profile := validSessionCreationProfile()
		payload, err := profile.CanonicalJSON()
		if err != nil {
			t.Fatalf("CanonicalJSON() error = %v", err)
		}
		encoded := string(payload)
		if !strings.Contains(encoded, `"version":4`) ||
			!strings.Contains(encoded, `"speed":"normal"`) ||
			!strings.Contains(encoded, `"profile_id":"00000000000000000000000000"`) {
			t.Fatalf("CanonicalJSON() = %s, want version 4 with normal speed and profile id", encoded)
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

	t.Run("Should change every digest when profile identity changes", func(t *testing.T) {
		t.Parallel()

		defaultProfile := validSessionCreationProfile()
		otherProfile := defaultProfile
		otherProfile.ProfileID = "01K3PROFILEIDENTITY00000001"
		options := SessionCreationOptions{
			SessionID:            "sess-profile-witness",
			NetworkOwnerKey:      "session:sess-profile-witness",
			NetworkParticipation: participation.LocalSpec(),
			SessionType:          "user",
		}

		defaultRef, err := defaultProfile.Ref()
		if err != nil {
			t.Fatalf("default Ref() error = %v", err)
		}
		otherRef, err := otherProfile.Ref()
		if err != nil {
			t.Fatalf("other Ref() error = %v", err)
		}
		defaultPolicy, err := defaultProfile.PolicySpecDigest()
		if err != nil {
			t.Fatalf("default PolicySpecDigest() error = %v", err)
		}
		otherPolicy, err := otherProfile.PolicySpecDigest()
		if err != nil {
			t.Fatalf("other PolicySpecDigest() error = %v", err)
		}
		defaultCreation, err := defaultProfile.CreationDigest(options)
		if err != nil {
			t.Fatalf("default CreationDigest() error = %v", err)
		}
		otherCreation, err := otherProfile.CreationDigest(options)
		if err != nil {
			t.Fatalf("other CreationDigest() error = %v", err)
		}
		if defaultRef == otherRef || defaultPolicy == otherPolicy || defaultCreation == otherCreation {
			t.Fatalf("profile identity did not bind every digest: refs=%t policies=%t creations=%t",
				defaultRef == otherRef, defaultPolicy == otherPolicy, defaultCreation == otherCreation)
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
		ProfileID:   DefaultProfileID,
		WorkspaceID: "workspace:test",
		CWD:         "/workspace",
		SandboxMode: SessionCreationSandboxNone,
		Permissions: "approve-all",
	}
}
