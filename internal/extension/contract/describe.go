package contract

import toolspkg "github.com/compozy/compozy/internal/tools"

// DescribePayload is the build-time contract emitted by an SDK describe process.
type DescribePayload struct {
	Name             string                                    `json:"name"`
	Version          string                                    `json:"version"`
	Description      string                                    `json:"description,omitempty"`
	Provides         []string                                  `json:"provides"`
	Permissions      []string                                  `json:"permissions"`
	RequiresEnv      []string                                  `json:"requires_env,omitempty"`
	Resources        DescribeResources                         `json:"resources"`
	Subprocess       DescribeSubprocess                        `json:"subprocess"`
	Tools            []toolspkg.ExtensionToolRuntimeDescriptor `json:"tools,omitempty"`
	HookEvents       []string                                  `json:"hook_events,omitempty"`
	WatchSourceKinds []string                                  `json:"watch_source_kinds,omitempty"`
	CommandGroups    []ExtensionCommandGroupSpec               `json:"command_groups,omitempty"`
	SDK              DescribeSDKInfo                           `json:"sdk"`
}

// DescribeResources declares source-relative static resource paths copied into a generation.
type DescribeResources struct {
	Skills     []string `json:"skills,omitempty"`
	Loops      []string `json:"loops,omitempty"`
	Agents     []string `json:"agents,omitempty"`
	Automation []string `json:"automation,omitempty"`
	Layouts    []string `json:"layouts,omitempty"`
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
