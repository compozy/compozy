// Package mcpfixture provides deterministic official-MCP-SDK fixtures for
// runtime and OAuth integration tests.
package mcpfixture

import "fmt"

// Profile selects an MCP interoperability behavior exposed by a Fixture.
type Profile string

const (
	// ProfileModern2026 implements the 2026-07-28 stateless server/discover flow.
	ProfileModern2026 Profile = "modern-2026"
	// ProfileLegacy2025 rejects server/discover so clients negotiate through initialize.
	ProfileLegacy2025 Profile = "legacy-2025"
	// ProfileInputRequired returns an MRTR input-required tool result.
	ProfileInputRequired Profile = "input-required"
)

func (p Profile) validate() error {
	switch p {
	case ProfileModern2026, ProfileLegacy2025, ProfileInputRequired:
		return nil
	default:
		return fmt.Errorf("mcp fixture: unsupported profile %q", p)
	}
}
