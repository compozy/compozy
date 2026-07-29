package extensionpkg

import (
	"slices"
	"strings"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	"github.com/compozy/compozy/internal/extension/surfaces"
	"github.com/compozy/compozy/internal/resources"
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

	var requestedPermissions []string
	var requestedResources surfaces.GrantRequest
	var err error
	if manifest != nil {
		requestedPermissions = normalizeUniqueStrings(manifest.Permissions.Requires)
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

	permissions := effectivePermissionGrants(source, requestedPermissions)
	security, err := derivedSecurityGrants(permissions)
	if err != nil {
		return capabilityGrant{}, err
	}

	return capabilityGrant{
		source:         source,
		permissions:    permissions,
		security:       security,
		resourceKinds:  resourceKinds,
		resourceScopes: resourceScopes,
	}, nil
}

func (g capabilityGrant) snapshot() EffectiveGrant {
	return EffectiveGrant{
		Permissions:    slices.Clone(g.permissions),
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

func effectivePermissionGrants(source ExtensionSource, requested []string) []string {
	requested = normalizeUniqueStrings(requested)
	policy := sourcePolicy(source)
	if policy.allowAllPermissions {
		return requested
	}
	if len(requested) == 0 || len(policy.allowedConsent) == 0 {
		return nil
	}

	var granted []string
	for _, method := range requested {
		contract, ok := extensioncontract.PermissionContractForMethod(method)
		if ok && capabilityGranted(policy.allowedConsent, contract.Capability) {
			granted = append(granted, method)
		}
	}
	return normalizeUniqueStrings(granted)
}

func derivedSecurityGrants(permissions []string) ([]string, error) {
	grants := make([]string, 0, len(permissions))
	for _, method := range permissions {
		contract, ok := extensioncontract.PermissionContractForMethod(method)
		if !ok {
			return nil, &ManifestValidationError{
				Field:   "permissions.requires",
				Value:   method,
				Message: unknownHostAPIPermissionMessage,
			}
		}
		grants = append(grants, contract.Capability)
	}
	return normalizeUniqueStrings(grants), nil
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
