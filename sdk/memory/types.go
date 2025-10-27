package memory

import (
	"github.com/compozy/compozy/engine/core"
	enginememory "github.com/compozy/compozy/engine/memory"
)

// ReferenceConfig mirrors engine memory references for SDK builders.
type ReferenceConfig = core.MemoryReference

// PrivacyScope exposes the privacy scope enumeration for builder configuration.
type PrivacyScope = enginememory.PrivacyScope

const (
	// PrivacyGlobalScope shares memory data across all tenants.
	PrivacyGlobalScope = enginememory.PrivacyGlobalScope
	// PrivacyUserScope restricts memory data to a single user.
	PrivacyUserScope = enginememory.PrivacyUserScope
	// PrivacySessionScope restricts memory data to a single session instance.
	PrivacySessionScope = enginememory.PrivacySessionScope
)
