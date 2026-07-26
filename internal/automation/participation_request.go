package automation

import (
	modelpkg "github.com/compozy/agh/internal/automation/model"
	"github.com/compozy/agh/internal/network/participation"
)

// NormalizeDirectTaskParticipation canonicalizes the authored participation
// request for a direct Automation-to-Task definition.
func NormalizeDirectTaskParticipation(
	request *participation.Request,
) (*participation.Request, error) {
	return modelpkg.NormalizeDirectTaskParticipation(request)
}

func cloneParticipationRequest(request *participation.Request) *participation.Request {
	return participation.CloneRequest(request)
}
