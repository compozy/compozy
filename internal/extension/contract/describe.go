package contract

import (
	hookspkg "github.com/compozy/compozy/internal/hooks"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// DescribePayload is the build-time contract emitted by an SDK describe process.
type DescribePayload struct {
	Name                 string                                    `json:"name"`
	Version              string                                    `json:"version"`
	Description          string                                    `json:"description,omitempty"`
	Provides             []string                                  `json:"provides"`
	Permissions          []string                                  `json:"permissions"`
	RequiresEnv          []string                                  `json:"requires_env,omitempty"`
	Profiles             []DescribeProfile                         `json:"profiles,omitempty"`
	Resources            DescribeResources                         `json:"resources"`
	Subprocess           DescribeSubprocess                        `json:"subprocess"`
	NetworkParticipation *DescribeNetworkParticipation             `json:"network_participation,omitempty"`
	Tools                []toolspkg.ExtensionToolRuntimeDescriptor `json:"tools,omitempty"`
	HookEvents           []DescribeHookEvent                       `json:"hook_events,omitempty"`
	WatchSourceKinds     []string                                  `json:"watch_source_kinds,omitempty"`
	CmdPaletteViews      []string                                  `json:"cmd_palette_views,omitempty"`
	CommandGroups        []ExtensionCommandGroupSpec               `json:"command_groups,omitempty"`
	SDK                  DescribeSDKInfo                           `json:"sdk"`
}

// DescribeProfile declares one profile seeded when an SDK-built extension is installed.
type DescribeProfile struct {
	Name        string                      `json:"name"`
	Color       string                      `json:"color,omitempty"`
	Icon        string                      `json:"icon,omitempty"`
	Emoji       string                      `json:"emoji,omitempty"`
	Defaults    DescribeProfileDefaults     `json:"defaults,omitzero"`
	Credentials []DescribeProfileCredential `json:"credentials,omitempty"`
}

// DescribeProfileDefaults declares profile defaults seeded at creation time.
type DescribeProfileDefaults struct {
	Agent    string `json:"agent,omitempty"`
	Provider string `json:"provider,omitempty"`
	Sandbox  string `json:"sandbox,omitempty"`
}

// DescribeProfileCredential declares one vault-backed setup requirement.
type DescribeProfileCredential struct {
	Provider string `json:"provider"`
	Slot     string `json:"slot"`
}

// DescribeResourcePath binds one static resource path to every profile or one named profile.
type DescribeResourcePath struct {
	Path    string `json:"path"`
	Profile string `json:"profile,omitempty"`
}

// DescribeHookEvent binds one supported hook event to every profile or one named profile.
type DescribeHookEvent struct {
	Event   hookspkg.HookEvent `json:"event"`
	Profile string             `json:"profile,omitempty"`
}

// DescribeNetworkParticipation declares the reachability control included in consent digests.
type DescribeNetworkParticipation struct {
	Required      bool     `json:"required"`
	Mode          string   `json:"mode"`
	ChannelScopes []string `json:"channel_scopes,omitempty"`
}

// DescribeResources declares source-relative static resource paths copied into a generation.
type DescribeResources struct {
	Skills     []DescribeResourcePath `json:"skills,omitempty"`
	Loops      []DescribeResourcePath `json:"loops,omitempty"`
	Agents     []DescribeResourcePath `json:"agents,omitempty"`
	Automation []DescribeResourcePath `json:"automation,omitempty"`
	Layouts    []DescribeResourcePath `json:"layouts,omitempty"`
	CmdPalette CmdPaletteConfig       `json:"cmd_palette,omitzero"`
}

// DescribeSubprocess declares the generated manifest's extension process entrypoint.
type DescribeSubprocess struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// DescribeSDKInfo identifies the SDK and compatibility floor that authored the payload.
type DescribeSDKInfo struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	ProtocolVersion   string `json:"protocol_version"`
	MinCompozyVersion string `json:"min_compozy_version"`
}
