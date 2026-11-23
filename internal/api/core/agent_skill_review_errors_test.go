package core_test

import (
	"fmt"
	"testing"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/core"
)

// Invariant (UT-038): every skill review domain error of `_dx.md` Errors keeps its stable code and
// HTTP status on every transport, also when wrapped with detail.
// Owning layer: api/core skill review error table.
func TestSkillReviewErrorStatus(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		err    error
		status int
	}{
		{agentinstance.ErrSkillProposeLessonRequired, 422},
		{agentinstance.ErrSkillProposeLessonTooLong, 422},
		{agentinstance.ErrSkillProposeLessonInvalid, 422},
		{agentinstance.ErrSkillProposeEvidenceRequired, 422},
		{agentinstance.ErrSkillProposeEvidenceNotFound, 422},
		{agentinstance.ErrSkillProposeNameTaken, 409},
		{agentinstance.ErrSkillReviewBaseMissing, 409},
		{agentinstance.ErrSkillReviewBaseUnsupported, 422},
		{agentinstance.ErrSkillProposeNoChange, 409},
		{agentinstance.ErrSkillReviewBaseStale, 409},
		{agentinstance.ErrSkillReviewRevertConflict, 409},
		{agentinstance.ErrSkillReviewOperatorOnly, 403},
		{agentinstance.ErrSkillReviewStale, 409},
		{agentinstance.ErrSkillInstallConflict, 409},
		{agentinstance.ErrSkillInstallBusy, 409},
		{agentinstance.ErrSkillReviewTargetExists, 409},
		{agentinstance.ErrSkillInstallConflictResolving, 409},
		{agentinstance.ErrSkillInstallConflictNotFound, 404},
		{agentinstance.ErrSkillProposePreviouslyRejected, 409},
		{agentinstance.ErrSkillProposeInvalidRemove, 422},
	} {
		t.Run("Should map "+tc.err.Error(), func(t *testing.T) {
			t.Parallel()
			code, status, ok := core.SkillReviewErrorStatus(fmt.Errorf("%w: detail", tc.err))
			if !ok || code != tc.err.Error() || status != tc.status {
				t.Fatalf("SkillReviewErrorStatus(%s) = %q %d %v; want %d", tc.err, code, status, ok, tc.status)
			}
		})
	}
}
