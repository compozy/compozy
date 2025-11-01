package memory

import memcore "github.com/compozy/compozy/engine/memory/core"

// PrivacyScope re-exports the engine memory privacy scope enumeration for SDKs.
type PrivacyScope = memcore.PrivacyScope

const (
	// PrivacyGlobalScope shares memory data across all tenants.
	PrivacyGlobalScope = memcore.PrivacyGlobalScope
	// PrivacyUserScope restricts memory data to a single user.
	PrivacyUserScope = memcore.PrivacyUserScope
	// PrivacySessionScope restricts memory data to a single session instance.
	PrivacySessionScope = memcore.PrivacySessionScope
)
