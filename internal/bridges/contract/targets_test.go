// Suite: bridge target snapshot wire contract
// Invariant: provider target snapshots carry stable identities and use deterministic lookup normalization.
// Boundary IN: provider-enumerated target snapshots.
// Boundary OUT: daemon directory persistence, ranking, and resolution, owned by directory suites.
package contract

import "testing"

func TestBridgeTargetTypeContract(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize and validate every target type", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			input BridgeTargetType
			want  BridgeTargetType
		}{
			{name: "Should normalize channel targets", input: " CHANNEL ", want: BridgeTargetTypeChannel},
			{name: "Should normalize user targets", input: " User ", want: BridgeTargetTypeUser},
			{name: "Should normalize room targets", input: " room ", want: BridgeTargetTypeRoom},
			{name: "Should normalize thread targets", input: " THREAD ", want: BridgeTargetTypeThread},
			{name: "Should normalize group targets", input: " Group ", want: BridgeTargetTypeGroup},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if got := test.input.Normalize(); got != test.want {
					t.Fatalf("BridgeTargetType(%q).Normalize() = %q, want %q", test.input, got, test.want)
				}
				if err := test.input.Validate(); err != nil {
					t.Fatalf("BridgeTargetType(%q).Validate() error = %v", test.input, err)
				}
			})
		}
	})

	t.Run("Should reject empty and unsupported target types", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			value BridgeTargetType
		}{
			{name: "Should reject an empty target type", value: ""},
			{name: "Should reject an unsupported target type", value: "conversation"},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if err := test.value.Validate(); err == nil {
					t.Fatalf("BridgeTargetType(%q).Validate() error = nil", test.value)
				}
			})
		}
	})
}

func TestBridgeTargetSnapshotContract(t *testing.T) {
	t.Parallel()

	t.Run("Should validate provider snapshots for every target type", func(t *testing.T) {
		t.Parallel()

		types := []BridgeTargetType{
			BridgeTargetTypeChannel,
			BridgeTargetTypeUser,
			BridgeTargetTypeRoom,
			BridgeTargetTypeThread,
			BridgeTargetTypeGroup,
		}
		for _, targetType := range types {
			t.Run("Should validate a "+string(targetType)+" snapshot", func(t *testing.T) {
				t.Parallel()

				snapshot := BridgeTargetSnapshot{
					CanonicalRoute: " provider:target-1 ",
					DisplayName:    " #Support ",
					TargetType:     targetType,
					Qualifier:      " Workspace ",
					Capabilities:   []string{"send", "thread"},
				}
				if err := snapshot.Validate(); err != nil {
					t.Fatalf("BridgeTargetSnapshot.Validate() error = %v", err)
				}
			})
		}
	})

	t.Run("Should reject incomplete provider snapshots", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			mutate func(*BridgeTargetSnapshot)
		}{
			{name: "Should require a canonical route", mutate: func(v *BridgeTargetSnapshot) { v.CanonicalRoute = "" }},
			{name: "Should require a display name", mutate: func(v *BridgeTargetSnapshot) { v.DisplayName = "" }},
			{
				name:   "Should require a supported target type",
				mutate: func(v *BridgeTargetSnapshot) { v.TargetType = "conversation" },
			},
			{
				name:   "Should require a searchable normalized name",
				mutate: func(v *BridgeTargetSnapshot) { v.DisplayName = " #@#@ " },
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				snapshot := BridgeTargetSnapshot{
					CanonicalRoute: "provider:target-1",
					DisplayName:    "Support",
					TargetType:     BridgeTargetTypeChannel,
				}
				test.mutate(&snapshot)
				if err := snapshot.Validate(); err == nil {
					t.Fatalf("BridgeTargetSnapshot.Validate(%s) error = nil", test.name)
				}
			})
		}
	})

	t.Run("Should require an instance id for snapshot requests", func(t *testing.T) {
		t.Parallel()

		t.Run("Should accept a populated instance id", func(t *testing.T) {
			t.Parallel()

			if err := (BridgeTargetSnapshotRequest{BridgeInstanceID: " brg-1 "}).Validate(); err != nil {
				t.Fatalf("BridgeTargetSnapshotRequest.Validate() error = %v", err)
			}
		})

		t.Run("Should reject an empty instance id", func(t *testing.T) {
			t.Parallel()

			if err := (BridgeTargetSnapshotRequest{}).Validate(); err == nil {
				t.Fatal("BridgeTargetSnapshotRequest.Validate(empty) error = nil")
			}
		})
	})
}

func TestBridgeTargetLookupNormalizationContract(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize target names and qualifiers deterministically", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			input string
			want  string
		}{
			{name: "Should trim and lowercase plain values", input: " Support Team ", want: "support team"},
			{name: "Should remove channel sigils", input: " ##General ", want: "general"},
			{name: "Should remove user sigils", input: " @@Alice ", want: "alice"},
			{name: "Should remove mixed leading sigils", input: " #@#Ops ", want: "ops"},
			{name: "Should preserve embedded sigils", input: "Team#One", want: "team#one"},
			{name: "Should normalize marker-only values to empty", input: " #@# ", want: ""},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if got := NormalizeBridgeTargetName(test.input); got != test.want {
					t.Fatalf("NormalizeBridgeTargetName(%q) = %q, want %q", test.input, got, test.want)
				}
				if got := NormalizeBridgeTargetQualifier(test.input); got != test.want {
					t.Fatalf("NormalizeBridgeTargetQualifier(%q) = %q, want %q", test.input, got, test.want)
				}
			})
		}
	})
}
