package tools

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

type Access string

const (
	AccessRead  Access = "read"
	AccessWrite Access = "write"
)

func AccessOf(d Descriptor) Access {
	if d.ReadOnly && !d.Destructive && !d.OpenWorld && !d.RequiresInteraction && d.Risk == RiskRead {
		return AccessRead
	}
	return AccessWrite
}

type CheckInPolicy struct {
	AuthorityApproved bool
	NoMainSession     bool
	Disabled          bool
	OccurrenceID      string
	InstanceID        string
	AllowWrites       []string
	MaxToolCalls      int
	MaxWriteCalls     int
	Ceiling           func(Descriptor, json.RawMessage) bool
	RunsUnattended    func(context.Context, Descriptor, json.RawMessage) bool
}

func checkInWritable(d Descriptor) bool {
	return AccessOf(d) == AccessWrite && (d.Backend.Kind == BackendMCP || IsCheckInWritable(d.ID))
}

func IsCheckInWritable(id ToolID) bool { return id == ToolIDNotify }
func (p *CheckInPolicy) denial(ctx context.Context, d Descriptor, input json.RawMessage) ReasonCode {
	if p.NoMainSession {
		return ReasonCheckInNoMainSession
	}
	if d.ID == ToolIDClarify || d.RequiresInteraction {
		return ReasonCheckInToolNotWritable
	}
	if p.Disabled &&
		(AccessOf(d) != AccessRead || d.Source.Kind != SourceBuiltin || strings.HasPrefix(string(d.ID), "compozy__terminal_")) {
		return ReasonPolicyDenied
	}
	if p.Ceiling != nil && !p.Ceiling(d, input) {
		return ReasonPolicyDenied
	}
	if AccessOf(d) == AccessRead {
		return ""
	}
	if d.Backend.Kind != BackendMCP && !IsCheckInWritable(d.ID) {
		return ReasonCheckInToolNotWritable
	}
	matched := false
	for _, selector := range p.AllowWrites {
		matched = matched || selector == string(d.ID) ||
			(strings.HasPrefix(selector, "mcp__") && strings.HasSuffix(selector, "__*") && strings.HasPrefix(string(d.ID), strings.TrimSuffix(selector, "*")))
	}
	if !matched {
		return ReasonCheckInWriteNotAllowed
	}
	if p.RunsUnattended == nil || !p.RunsUnattended(ctx, d, input) {
		return ReasonCheckInWriteNeedsApproval
	}
	return ""
}

type CheckInAttempt struct {
	OccurrenceID string
	ToolID       ToolID
	Backend      BackendKind
	MCPServer    string
	Access       Access
	DenyReason   string
	ArgsPreview  string
}
type CheckInDecision struct {
	Seq     int
	Allowed bool
	Reason  string
}
type CheckInRecorder interface {
	Begin(context.Context, CheckInAttempt, CheckInPolicy) (CheckInDecision, error)
	Finish(context.Context, string, int, string, string, time.Duration) error
}
