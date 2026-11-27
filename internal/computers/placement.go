package computers

import (
	"cmp"
	"math"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/remoteproto"
)

const LocalID remoteproto.ComputerID = "local"
const AutoPlacement = "auto"

const maxResourceSampleAge = 15 * time.Second

type ResourceSample struct {
	CPUCount     int
	CPUUtil      *float64
	MemTotal     *uint64
	MemAvailable *uint64
	SampledAt    time.Time
}

type PlacementCandidate struct {
	ComputerID  remoteproto.ComputerID
	Name        string
	Enabled     bool
	Connected   bool
	Weight      int
	WorkspaceID string
	Resources   ResourceSample
}

func SelectPlacement(now time.Time, candidates []PlacementCandidate) (PlacementCandidate, error) {
	var best PlacementCandidate
	var bestScore float64
	reasons := make(map[string]string)
	for _, candidate := range candidates {
		if reason := candidate.exclusion(); reason != "" {
			reasons[string(candidate.ComputerID)] = reason
			continue
		}
		score, reason := candidate.score(now)
		if reason != "" {
			reasons[string(candidate.ComputerID)] = reason
			continue
		}
		if best.ComputerID == "" || cmp.Or(cmp.Compare(score, bestScore), candidate.tieBreak(best)) > 0 {
			best, bestScore = candidate, score
		}
	}
	if best.ComputerID == "" {
		return best, NoEligibleComputer(reasons)
	}
	return best, nil
}

func NoEligibleComputer(reasons map[string]string) error {
	return &Failure{
		Status: http.StatusConflict, Code: "no_eligible_computer", Message: "No computer can run this project.",
		Details: reasons,
	}
}

func (c PlacementCandidate) exclusion() string {
	switch {
	case !c.Enabled && c.ComputerID == LocalID:
		return "local_execution_disabled"
	case !c.Enabled:
		return "computer_disabled"
	case !c.Connected:
		return "computer_offline"
	case c.Weight <= 0:
		return "manual_only"
	case c.WorkspaceID == "":
		return "project_not_found"
	default:
		return ""
	}
}

func (c PlacementCandidate) score(now time.Time) (float64, string) {
	s := c.Resources
	if s.SampledAt.IsZero() || now.Sub(s.SampledAt) > maxResourceSampleAge || s.CPUCount <= 0 ||
		s.CPUUtil == nil || s.MemTotal == nil || s.MemAvailable == nil || *s.MemTotal == 0 ||
		math.IsNaN(*s.CPUUtil) || *s.CPUUtil < 0 || *s.CPUUtil > 1 || *s.MemAvailable > *s.MemTotal {
		// Fallback scores stay below every fresh eligible score and order by weight.
		return -1 / float64(c.Weight), ""
	}
	free := float64(*s.MemAvailable) / float64(*s.MemTotal)
	if *s.CPUUtil >= 0.95 {
		return 0, "cpu_busy"
	}
	if free <= 0.05 {
		return 0, "memory_busy"
	}
	return float64(c.Weight) * float64(s.CPUCount) * (1 - *s.CPUUtil) * free, ""
}

func (c PlacementCandidate) tieBreak(other PlacementCandidate) int {
	if c.ComputerID == LocalID {
		return 1
	}
	if other.ComputerID == LocalID {
		return -1
	}
	return cmp.Or(cmp.Compare(other.Name, c.Name), cmp.Compare(other.ComputerID, c.ComputerID))
}
