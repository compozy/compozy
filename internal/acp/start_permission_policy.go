package acp

import compozyconfig "github.com/compozy/compozy/internal/config"

// StartPermissionPolicy binds the session-owned approval policy to a process launch.
type StartPermissionPolicy struct {
	ExecutionPolicy string
	Rules           []compozyconfig.PermissionRule
	Denials         *PermissionDenials
}

func (o StartOpts) ExecutionPolicyValue() string {
	if o.PermissionPolicy == nil {
		return ""
	}
	return o.PermissionPolicy.ExecutionPolicy
}

func (o *StartOpts) SetExecutionPolicy(value string) {
	if o.PermissionPolicy == nil {
		o.PermissionPolicy = &StartPermissionPolicy{}
	}
	o.PermissionPolicy.ExecutionPolicy = value
}

func (o StartOpts) PermissionRulesValue() []compozyconfig.PermissionRule {
	if o.PermissionPolicy == nil {
		return nil
	}
	return o.PermissionPolicy.Rules
}
