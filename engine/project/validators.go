package project

import (
	"errors"
	"fmt"
)

type WorkflowsValidator struct {
	workflows []*WorkflowSourceConfig
}

func NewWorkflowsValidator(workflows []*WorkflowSourceConfig) *WorkflowsValidator {
	return &WorkflowsValidator{workflows: workflows}
}

func (v *WorkflowsValidator) Validate() error {
	if len(v.workflows) == 0 {
		return errors.New("no workflows defined in project")
	}
	for _, wf := range v.workflows {
		if wf.Source == "" {
			return fmt.Errorf("workflow %s source is empty", wf.Source)
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// WebhookSlugsValidator - Validates uniqueness of webhook slugs across workflows
// -----------------------------------------------------------------------------

type WebhookSlugsValidator struct {
	slugs []string
}

func NewWebhookSlugsValidator(slugs []string) *WebhookSlugsValidator {
	return &WebhookSlugsValidator{slugs: slugs}
}

func (v *WebhookSlugsValidator) Validate() error {
	seen := make(map[string]struct{}, len(v.slugs))
	for _, slug := range v.slugs {
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			return fmt.Errorf("duplicate webhook slug '%s'", slug)
		}
		seen[slug] = struct{}{}
	}
	return nil
}
