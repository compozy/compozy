package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/network/participation"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

func TestSessionCreationProfileShouldBindRuntimePolicyToVersionedIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should persist typed runtime defaults and scope in the canonical version five profile", func(t *testing.T) {
		t.Parallel()

		profile := validSessionCreationProfile()
		profile.ACPOptions = []SessionACPOptionSelection{{ID: "thinking", BoolValue: new(true)}}
		payload, err := profile.CanonicalJSON()
		if err != nil {
			t.Fatalf("CanonicalJSON() error = %v", err)
		}
		encoded := string(payload)
		if !strings.Contains(encoded, `"version":5`) ||
			!strings.Contains(encoded, `"speed":"normal"`) ||
			!strings.Contains(encoded, `"acp_options":[{"id":"thinking","bool_value":true}]`) ||
			!strings.Contains(encoded, `"profile_id":"00000000000000000000000000"`) ||
			!strings.Contains(encoded, `"scope":"workspace"`) {
			t.Fatalf("CanonicalJSON() = %s, want version 5 with typed runtime defaults and scope", encoded)
		}
		var decoded SessionCreationProfile
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("json.Unmarshal(CanonicalJSON()) error = %v", err)
		}
		if decoded.Version != 5 || decoded.Speed != speedpkg.SpeedNormal || decoded.Scope != SessionScopeWorkspace ||
			len(decoded.ACPOptions) != 1 || decoded.ACPOptions[0].ID != "thinking" ||
			decoded.ACPOptions[0].BoolValue == nil || !*decoded.ACPOptions[0].BoolValue {
			t.Fatalf("decoded creation profile = %#v, want structural typed runtime fields", decoded)
		}
	})

	t.Run("Should accept a global profile without workspace identity", func(t *testing.T) {
		t.Parallel()

		profile := validSessionCreationProfile()
		profile.Scope = SessionScopeGlobal
		profile.WorkspaceID = ""
		if err := profile.Validate(); err != nil {
			t.Fatalf("Validate(global) error = %v", err)
		}
		profile.WorkspaceID = "workspace:test"
		if err := profile.Validate(); err == nil || !strings.Contains(err.Error(), "cannot bind a workspace") {
			t.Fatalf("Validate(global workspace) error = %v, want location rejection", err)
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

	t.Run("Should change the immutable profile reference when an ACP option changes", func(t *testing.T) {
		t.Parallel()

		withoutOption := validSessionCreationProfile()
		withOption := withoutOption
		withOption.ACPOptions = []SessionACPOptionSelection{{ID: "thinking", BoolValue: new(true)}}
		withoutRef, err := withoutOption.Ref()
		if err != nil {
			t.Fatalf("without option Ref() error = %v", err)
		}
		withRef, err := withOption.Ref()
		if err != nil {
			t.Fatalf("with option Ref() error = %v", err)
		}
		if withoutRef == withRef {
			t.Fatalf("profile refs = %q, want ACP-option-specific identities", withoutRef)
		}
	})

	t.Run("Should change every derived identity when profile identity changes", func(t *testing.T) {
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
		Scope:       SessionScopeWorkspace,
		WorkspaceID: "workspace:test",
		CWD:         "/workspace",
		SandboxMode: SessionCreationSandboxNone,
		Permissions: "approve-all",
	}
}
