package udsapi

import "github.com/compozy/agh/internal/api/core"

// ExtensionService exposes daemon-backed extension management to the UDS API.
type ExtensionService = core.ExtensionService

func extensionStatusCode(err error) int {
	return core.ExtensionStatusCode(err)
}
