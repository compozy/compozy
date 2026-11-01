package agentinstance

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type rotationRuntime struct {
	steps       []string
	decision    RotationDecision
	cleanupErr  error
	rotationErr error
}

var _ MainSessionRotator = (*rotationRuntime)(nil)

func (r *rotationRuntime) Cleanup(context.Context, Instance) error {
	r.steps = append(r.steps, "cleanup")
	return r.cleanupErr
}
func (r *rotationRuntime) Eligible(context.Context, Instance) (RotationDecision, error) {
	r.steps = append(r.steps, "eligible")
	return r.decision, nil
}
func (r *rotationRuntime) Rotate(context.Context, Instance, RotationDecision) error {
	r.steps = append(r.steps, "rotate")
	return r.rotationErr
}

func TestMainSessionRotation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		state      string
		eligible   bool
		stale      bool
		cleanupErr error
		want       []string
		wantErr    bool
	}{
		{name: "Should clean predecessors before rotating an eligible main", state: "active", eligible: true, want: []string{"cleanup", "eligible", "rotate"}},
		{name: "Should leave an ineligible main untouched", state: "active", want: []string{"cleanup", "eligible"}},
		{name: "Should clean predecessors without rotating paused instances", state: "paused", eligible: true, want: []string{"cleanup"}},
		{name: "Should retry cleanup before considering another rotation", state: "active", cleanupErr: errors.New("stop failed"), want: []string{"cleanup"}, wantErr: true},
		{name: "Should reject a decision for another main", state: "active", eligible: true, stale: true, want: []string{"cleanup", "eligible"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runtime := &rotationRuntime{decision: RotationDecision{Eligible: tc.eligible, Publication: RotationPublication{InstanceID: "instance", PredecessorID: "main"}}, cleanupErr: tc.cleanupErr}
			if tc.stale {
				runtime.decision.Publication.PredecessorID = "old"
			}
			service := &Service{rotator: runtime}
			err := service.rotateMain(t.Context(), Instance{ID: "instance", MainSessionID: "main", State: tc.state})
			if (err != nil) != tc.wantErr || !reflect.DeepEqual(runtime.steps, tc.want) {
				t.Fatalf("rotation = %v, steps %v", err, runtime.steps)
			}
		})
	}
}
