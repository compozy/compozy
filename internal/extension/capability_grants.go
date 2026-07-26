package extensionpkg

import (
	"slices"
	"strings"

	"github.com/compozy/agh/internal/extension/surfaces"
	"github.com/compozy/agh/internal/resources"
)

func (c *CapabilityChecker) lookup(extName string) capabilityGrant {
	if c == nil {
		return capabilityGrant{}
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.grants == nil {
		return capabilityGrant{}
	}
	return c.grants[strings.TrimSpace(extName)]
}

func (c *CapabilityChecker) resolve(
	source ExtensionSource,
	manifest *Manifest,
	sessionMaxScope resources.ResourceScopeKind,
) (capabilityGrant, error) {
	c.mu.RLock()
	policy := cloneResourcePolicy(c.resourcePolicy)
	c.mu.RUnlock()

	var requestedActions []string
	var requestedSecurity []string
	var requestedResources surfaces.GrantRequest
	var err error
	if manifest != nil {
		requestedActions = normalizeUniqueStrings(manifest.Actions.Requires)
		requestedSecurity = normalizeUniqueStrings(manifest.Security.Capabilities)
		requestedResources, err = surfaces.ResolveManifestRequest(
			manifest.Resources.Publish.Families,
			manifest.Resources.Publish.MaxScope,
		)
		if err != nil {
			return capabilityGrant{}, err
		}
	}

	resourceKinds, resourceScopes, err := effectiveResourceGrants(
		source,
		policy,
		requestedResources,
		sessionMaxScope,
	)
	if err != nil {
		return capabilityGrant{}, err
	}

	return capabilityGrant{
		source:         source,
		actions:        effectiveActionGrants(source, requestedActions),
		security:       effectiveSecurityGrants(source, requestedSecurity),
		resourceKinds:  resourceKinds,
		resourceScopes: resourceScopes,
	}, nil
}

func (g capabilityGrant) snapshot() EffectiveGrant {
	return EffectiveGrant{
		Actions:        slices.Clone(g.actions),
		Security:       slices.Clone(g.security),
		ResourceKinds:  slices.Clone(g.resourceKinds),
		ResourceScopes: slices.Clone(g.resourceScopes),
	}
}

func newCapabilityDeniedError(method string, required []string, granted []string) error {
	return &ErrCapabilityDenied{
		Data: CapabilityDeniedData{
			Method:   strings.TrimSpace(method),
			Required: normalizeUniqueStrings(required),
			Granted:  normalizeUniqueStrings(granted),
		},
	}
}

func effectiveActionGrants(source ExtensionSource, requested []string) []string {
	requested = normalizeUniqueStrings(requested)
	policy := sourcePolicy(source)
	if policy.allowAllActions {
		return requested
	}
	if len(requested) == 0 || len(policy.allowedActions) == 0 {
		return nil
	}

	var granted []string
	for _, method := range requested {
		if slices.Contains(policy.allowedActions, method) {
			granted = append(granted, method)
		}
	}
	return normalizeUniqueStrings(granted)
}

func effectiveSecurityGrants(source ExtensionSource, requested []string) []string {
	requested = normalizeUniqueStrings(requested)
	policy := sourcePolicy(source)
	if policy.allowAllSecurity {
		return requested
	}
	if len(requested) == 0 || len(policy.allowedSecurity) == 0 {
		return nil
	}

	var granted []string
	for _, request := range requested {
		if ceilingAllowsRequestedGrant(policy.allowedSecurity, request) {
			granted = append(granted, request)
			continue
		}

		for _, allowed := range policy.allowedSecurity {
			if capabilityGrantSuperset(request, allowed) {
				granted = append(granted, allowed)
			}
		}
	}
	return normalizeUniqueStrings(granted)
}

func capabilityGranted(grants []string, capability string) bool {
	required := strings.TrimSpace(capability)
	if required == "" {
		return false
	}
	for _, grant := range grants {
		if capabilityGrantSuperset(grant, required) {
			return true
		}
	}
	return false
}

func ceilingAllowsRequestedGrant(ceiling []string, requested string) bool {
	for _, allowed := range ceiling {
		if capabilityGrantSuperset(allowed, requested) {
			return true
		}
	}
	return false
}

func capabilityGrantSuperset(grant string, requested string) bool {
	grant = strings.TrimSpace(grant)
	requested = strings.TrimSpace(requested)
	switch {
	case grant == "", requested == "":
		return false
	case grant == "*":
		return true
	case requested == "*":
		return grant == "*"
	}

	grantParts := strings.Split(grant, ".")
	requestedParts := strings.Split(requested, ".")
	if len(grantParts) != len(requestedParts) {
		return false
	}

	for idx, part := range grantParts {
		if part == "*" {
			continue
		}
		if requestedParts[idx] == "*" {
			return false
		}
		if part != requestedParts[idx] {
			return false
		}
	}
	return true
}

func normalizeUniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	dst := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		dst = append(dst, trimmed)
	}
	if len(dst) == 0 {
		return nil
	}
	slices.Sort(dst)
	return dst
}
