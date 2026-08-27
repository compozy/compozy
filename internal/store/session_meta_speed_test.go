package store

import (
	"testing"
	"time"

	"github.com/compozy/compozy/internal/network/participation"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

func TestSessionMetaValidateSpeed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		speed      speedpkg.Speed
		resolution *speedpkg.Resolution
		wantErr    bool
	}{
		{name: "Should accept legacy metadata without speed"},
		{
			name:  "Should accept a canonical speed without a resolution",
			speed: speedpkg.SpeedNormal,
		},
		{
			name:  "Should accept a matching speed resolution",
			speed: speedpkg.SpeedFast,
			resolution: &speedpkg.Resolution{
				Requested: speedpkg.SpeedFast,
				Status:    speedpkg.ResolutionApplied,
			},
		},
		{
			name:    "Should reject an invalid speed",
			speed:   speedpkg.Speed("turbo"),
			wantErr: true,
		},
		{
			name: "Should reject a resolution without speed",
			resolution: &speedpkg.Resolution{
				Requested: speedpkg.SpeedFast,
				Status:    speedpkg.ResolutionApplied,
			},
			wantErr: true,
		},
		{
			name:  "Should reject a mismatched resolution",
			speed: speedpkg.SpeedNormal,
			resolution: &speedpkg.Resolution{
				Requested: speedpkg.SpeedFast,
				Status:    speedpkg.ResolutionApplied,
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
			meta := SessionMeta{
				ID:                        "sess-speed",
				AgentName:                 "coder",
				WorkspaceID:               "ws-speed",
				SessionMetaPlacementState: NewSessionMetaPlacement(SessionScopeWorkspace, participation.LocalSpec()),
				State:                     "active",
				RuntimeStatus:             SessionRuntimeReady,
				Speed:                     test.speed,
				SpeedResolution:           speedpkg.CloneResolution(test.resolution),
				CreatedAt:                 now,
				UpdatedAt:                 now,
			}

			err := meta.Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("SessionMeta.Validate() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}
