package loop

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// EffectContextRequest contains immutable facts bound before an effect template is resolved.
type EffectContextRequest struct {
	Run           *Run
	Trigger       EffectTrigger
	Generation    int
	NodeID        NodeID
	ItemIndex     int
	Attempt       int
	NextAttemptAt *time.Time
	Disposition   AttemptDisposition
	Failure       *ClassifiedFailure
	Quarantine    json.RawMessage
}

type effectIdentityContext struct {
	Scope       string        `json:"scope"`
	WorkspaceID WorkspaceID   `json:"workspace_id"`
	LoopName    string        `json:"loop_name"`
	LoopRunID   RunID         `json:"loop_run_id"`
	Generation  int           `json:"generation"`
	NodeID      NodeID        `json:"node_id,omitempty"`
	ItemIndex   int           `json:"item_index"`
	Trigger     EffectTrigger `json:"trigger"`
}

type effectAttemptContext struct {
	Number        int                `json:"number"`
	NextAttemptAt *time.Time         `json:"next_attempt_at,omitempty"`
	Disposition   AttemptDisposition `json:"disposition,omitempty"`
}

type effectLinksContext struct {
	Run      string `json:"run"`
	Resume   string `json:"resume,omitempty"`
	Approval string `json:"approval,omitempty"`
}

type effectTemplateContext struct {
	Identity   effectIdentityContext `json:"identity"`
	Failure    *ClassifiedFailure    `json:"failure,omitempty"`
	Quarantine json.RawMessage       `json:"quarantine,omitempty"`
	Attempt    effectAttemptContext  `json:"attempt"`
	Links      effectLinksContext    `json:"links"`
}

// BuildEffectContext pre-binds stable identity, failure, attempt, and decision links.
func BuildEffectContext(req EffectContextRequest) (map[string]any, error) {
	if err := req.validate(); err != nil {
		return nil, err
	}
	scope := effectScopeLoop
	if req.NodeID != "" {
		scope = effectScopeNode
	}
	runLink := effectRunLink(req.Run.WorkspaceID, req.Run.ID)
	context := effectTemplateContext{
		Identity: effectIdentityContext{
			Scope: scope, WorkspaceID: req.Run.WorkspaceID, LoopName: req.Run.LoopName,
			LoopRunID: req.Run.ID, Generation: req.Generation, NodeID: req.NodeID,
			ItemIndex: req.ItemIndex, Trigger: req.Trigger,
		},
		Failure:    cloneClassifiedFailure(req.Failure),
		Quarantine: append(json.RawMessage(nil), req.Quarantine...),
		Attempt: effectAttemptContext{
			Number: req.Attempt, NextAttemptAt: cloneTimePointer(req.NextAttemptAt),
			Disposition: req.Disposition,
		},
		Links: effectLinksContext{Run: runLink},
	}
	if req.NodeID != "" {
		context.Links.Resume = effectDecisionLink(runLink, req, "resume")
		context.Links.Approval = effectDecisionLink(runLink, req, "approval")
	}
	raw, err := json.Marshal(context)
	if err != nil {
		return nil, fmt.Errorf("loop: marshal effect context: %w", err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("loop: decode effect context: %w", err)
	}
	return result, nil
}

func (r EffectContextRequest) validate() error {
	if r.Run == nil || r.Run.ID == "" || r.Run.WorkspaceID == "" || strings.TrimSpace(r.Run.LoopName) == "" {
		return fmt.Errorf("%w: effect run identity is incomplete", ErrValidation)
	}
	if !r.Trigger.Valid() || r.Generation < 1 || r.ItemIndex < 0 || r.Attempt < 0 {
		return fmt.Errorf("%w: effect context identity is invalid", ErrValidation)
	}
	if r.Trigger.NodeScoped() && r.NodeID == "" {
		return fmt.Errorf("%w: node effect requires node_id", ErrValidation)
	}
	if !r.Trigger.NodeScoped() && r.NodeID != "" {
		return fmt.Errorf("%w: loop effect must not carry node_id", ErrValidation)
	}
	if len(r.Quarantine) > 0 && !json.Valid(r.Quarantine) {
		return fmt.Errorf("%w: effect quarantine context must be valid JSON", ErrValidation)
	}
	return nil
}

func effectRunLink(workspaceID WorkspaceID, runID RunID) string {
	return "/api/workspaces/" + url.PathEscape(strings.TrimSpace(string(workspaceID))) +
		"/loop-runs/" + url.PathEscape(strings.TrimSpace(string(runID)))
}

func effectDecisionLink(runLink string, req EffectContextRequest, action string) string {
	query := url.Values{}
	query.Set("action", action)
	query.Set("generation", strconv.Itoa(req.Generation))
	query.Set(metadataItemIndexKey, strconv.Itoa(req.ItemIndex))
	query.Set("node_id", string(req.NodeID))
	return runLink + "?" + query.Encode()
}

func cloneClassifiedFailure(value *ClassifiedFailure) *ClassifiedFailure {
	if value == nil {
		return nil
	}
	cloned := *value
	if value.RetryAfter != nil {
		retryAfter := *value.RetryAfter
		cloned.RetryAfter = &retryAfter
	}
	return &cloned
}
