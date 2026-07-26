package extensionpkg

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/resources"
)

const (
	hostAPIAutomationJobsPath              = "automation/jobs"
	hostAPIAutomationJobsCreatePath        = "automation/jobs/create"
	hostAPIBridgesInstancesListPath        = "bridges/instances/list"
	hostAPIBridgesInstancesReportStatePath = "bridges/instances/report_state"
	hostAPIMemoryForgetPath                = "memory/forget"
	hostAPIMemoryRecallPath                = "memory/recall"
	hostAPIListLogsPath                    = "logs/list"
	hostAPIResourcesListPath               = "resources/list"
	hostAPIResourcesSnapshotPath           = "resources/snapshot"
	hostAPISandboxExecPath                 = "sandbox/exec"
	hostAPISessionsCreatePath              = "sessions/create"
	hostAPISessionsEventsPath              = "sessions/events"
	hostAPISkillsListPath                  = "skills/list"
)

const (
	capabilityModelsListPath    = "models/list"
	capabilityModelsRefreshPath = "models/refresh"
	capabilityTasksKey          = "tasks"
	capabilityTasksCreatePath   = "tasks/create"
)

const (
	capabilityAutomationReadPath           = "automation.read"
	capabilityAutomationWritePath          = "automation.write"
	capabilityAutomationJobsDeletePath     = "automation/jobs/delete"
	capabilityAutomationJobsGetPath        = "automation/jobs/get"
	capabilityAutomationJobsRunsPath       = "automation/jobs/runs"
	capabilityAutomationJobsTriggerPath    = "automation/jobs/trigger"
	capabilityAutomationJobsUpdatePath     = "automation/jobs/update"
	capabilityAutomationRunsPath           = "automation/runs"
	capabilityAutomationTriggersPath       = "automation/triggers"
	capabilityAutomationTriggersCreatePath = "automation/triggers/create"
	capabilityAutomationTriggersDeletePath = "automation/triggers/delete"
	capabilityAutomationTriggersFirePath   = "automation/triggers/fire"
	capabilityAutomationTriggersGetPath    = "automation/triggers/get"
	capabilityAutomationTriggersRunsPath   = "automation/triggers/runs"
	capabilityAutomationTriggersUpdatePath = "automation/triggers/update"
	capabilityBridgeReadPath               = "bridge.read"
	capabilityBridgeWritePath              = "bridge.write"
	capabilityBridgesInstancesGetPath      = "bridges/instances/get"
	capabilityBridgesMessagesIngestPath    = "bridges/messages/ingest"
	capabilityBundledKey                   = "bundled"
	capabilityHeartbeatReadPath            = "heartbeat.read"
	capabilityHeartbeatWritePath           = "heartbeat.write"
	capabilityLogsReadPath                 = "logs.read"
	capabilityMemoryReadPath               = "memory.read"
	capabilityMemoryWritePath              = "memory.write"
	capabilityMemoryStorePath              = "memory/store"
	capabilityModelReadPath                = "model.read"
	capabilityModelWritePath               = "model.write"
	capabilityModelsStatusPath             = "models/status"
	capabilityNetworkReadPath              = "network.read"
	capabilityNetworkWritePath             = "network.write"
	capabilityObserveReadPath              = "observe.read"
	capabilityObserveHealthPath            = "observe/health"
	capabilityResourceReadPath             = "resources.read"
	capabilityResourceWritePath            = "resources.write"
	capabilityResourcesGetPath             = "resources/get"
	capabilitySandboxExecPath              = "sandbox.exec"
	capabilitySandboxInfoPath              = "sandbox/info"
	capabilitySandboxListPath              = "sandbox/list"
	capabilitySessionReadPath              = "session.read"
	capabilitySessionWritePath             = "session.write"
	capabilitySessionsListPath             = "sessions/list"
	capabilitySessionsPromptPath           = "sessions/prompt"
	capabilitySessionsStatusPath           = "sessions/status"
	capabilitySessionsStopPath             = "sessions/stop"
	capabilitySkillsReadPath               = "skills.read"
	capabilitySoulReadPath                 = "soul.read"
	capabilitySoulWritePath                = "soul.write"
	capabilityTaskReadPath                 = "task.read"
	capabilityTaskWritePath                = "task.write"
	capabilityTasksCancelPath              = "tasks/cancel"
	capabilityTasksDashboardPath           = "tasks/dashboard"
	capabilityTasksGetPath                 = "tasks/get"
	capabilityTasksInboxPath               = "tasks/inbox"
	capabilityTasksRunsPath                = "tasks/runs"
	capabilityTasksRunsAttachSessionPath   = "tasks/runs/attach_session"
	capabilityTasksRunsCancelPath          = "tasks/runs/cancel"
	capabilityTasksRunsCompletePath        = "tasks/runs/complete"
	capabilityTasksRunsEnqueuePath         = "tasks/runs/enqueue"
	capabilityTasksRunsFailPath            = "tasks/runs/fail"
	capabilityTasksRunsGetPath             = "tasks/runs/get"
	capabilityTasksRunsStartPath           = "tasks/runs/start"
	capabilityTasksTimelinePath            = "tasks/timeline"
	capabilityTasksTreePath                = "tasks/tree"
	capabilityTasksUpdatePath              = "tasks/update"
	capabilityToolReadPath                 = "tool.read"
	capabilityUserKey                      = "user"
	capabilityWorkspaceKey                 = "workspace"
)

const (
	// CapabilityDeniedCode is the protocol-equivalent code for denied extension
	// capabilities and Host API actions.
	CapabilityDeniedCode = -32001
)

var (
	marketplaceSecurityCeiling = []string{
		capabilityMemoryReadPath,
		capabilityLogsReadPath,
		capabilityObserveReadPath,
		capabilitySessionReadPath,
		capabilitySkillsReadPath,
		capabilityToolReadPath,
	}
)

// ExtensionSource identifies where an extension was installed from.
type ExtensionSource int

const (
	// SourceBundled identifies built-in extensions shipped with the daemon.
	SourceBundled ExtensionSource = iota
	// SourceUser identifies user-installed extensions trusted by the operator.
	SourceUser
	// SourceWorkspace identifies workspace-scoped extensions trusted by the project.
	SourceWorkspace
	// SourceMarketplace identifies marketplace-installed extensions subject to
	// restricted default grants until an explicit allowlist exists.
	SourceMarketplace
)

// String returns the persisted text form for one extension source tier.
func (s ExtensionSource) String() string {
	switch s {
	case SourceBundled:
		return capabilityBundledKey
	case SourceUser:
		return capabilityUserKey
	case SourceWorkspace:
		return capabilityWorkspaceKey
	case SourceMarketplace:
		return sourceMarketplaceName
	default:
		return ""
	}
}

// CapabilityDeniedData is the structured data for capability-denied failures.
type CapabilityDeniedData struct {
	Method   string   `json:"method"`
	Required []string `json:"required"`
	Granted  []string `json:"granted"`
}

// ErrCapabilityDenied reports that an extension attempted a method or
// capability outside its effective grants.
type ErrCapabilityDenied struct {
	Data CapabilityDeniedData
}

// Error returns the protocol-aligned capability denied message.
func (e *ErrCapabilityDenied) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Data.Method) == "" {
		return "capability_denied"
	}
	if len(e.Data.Required) == 0 {
		return fmt.Sprintf("capability_denied: %s", e.Data.Method)
	}
	return fmt.Sprintf(
		"capability_denied: %s requires %v (granted %v)",
		e.Data.Method,
		e.Data.Required,
		e.Data.Granted,
	)
}

// Code returns the protocol-equivalent error code for capability denials.
func (e *ErrCapabilityDenied) Code() int {
	return CapabilityDeniedCode
}

// CapabilityChecker tracks effective grants per extension and evaluates
// capability checks for hook dispatch and Host API calls.
type CapabilityChecker struct {
	mu             sync.RWMutex
	grants         map[string]capabilityGrant
	resourcePolicy aghconfig.ExtensionsResourcesConfig
}

type capabilityGrant struct {
	source         ExtensionSource
	actions        []string
	security       []string
	resourceKinds  []resources.ResourceKind
	resourceScopes []resources.ResourceScopeKind
}

// EffectiveGrant is the daemon-derived grant snapshot for one extension session.
type EffectiveGrant struct {
	Actions        []string
	Security       []string
	ResourceKinds  []resources.ResourceKind
	ResourceScopes []resources.ResourceScopeKind
}

// Register records one extension's effective grants by applying the source-tier
// ceiling before intersecting it with the manifest requests.
func (c *CapabilityChecker) Register(extName string, source ExtensionSource, manifest *Manifest) {
	if _, err := c.RegisterForSession(extName, source, manifest, resources.ResourceScopeKindGlobal); err != nil {
		return
	}
}

// RegisterForSession records one extension's effective grants for the supplied session scope ceiling.
func (c *CapabilityChecker) RegisterForSession(
	extName string,
	source ExtensionSource,
	manifest *Manifest,
	sessionMaxScope resources.ResourceScopeKind,
) (EffectiveGrant, error) {
	if c == nil {
		return EffectiveGrant{}, nil
	}

	name := strings.TrimSpace(extName)
	if name == "" {
		return EffectiveGrant{}, nil
	}

	grant, err := c.resolve(source, manifest, sessionMaxScope)
	if err != nil {
		return EffectiveGrant{}, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.grants == nil {
		c.grants = make(map[string]capabilityGrant)
	}
	c.grants[name] = grant
	return grant.snapshot(), nil
}

// Resolve computes one daemon-derived grant snapshot without storing it.
func (c *CapabilityChecker) Resolve(
	source ExtensionSource,
	manifest *Manifest,
	sessionMaxScope resources.ResourceScopeKind,
) (EffectiveGrant, error) {
	if c == nil {
		return EffectiveGrant{}, nil
	}

	grant, err := c.resolve(source, manifest, sessionMaxScope)
	if err != nil {
		return EffectiveGrant{}, err
	}
	return grant.snapshot(), nil
}

// Grant returns the stored effective grant snapshot for one extension.
func (c *CapabilityChecker) Grant(extName string) EffectiveGrant {
	return c.lookup(extName).snapshot()
}

// SetResourcePolicy installs the operator-configured extension resource policy.
func (c *CapabilityChecker) SetResourcePolicy(policy aghconfig.ExtensionsResourcesConfig) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.resourcePolicy = cloneResourcePolicy(policy)
}

// Unregister removes any effective grants tracked for one extension.
func (c *CapabilityChecker) Unregister(extName string) {
	if c == nil {
		return
	}

	name := strings.TrimSpace(extName)
	if name == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.grants == nil {
		return
	}
	delete(c.grants, name)
}

// Check reports whether extName has the requested security capability.
func (c *CapabilityChecker) Check(extName, capability string) error {
	required := strings.TrimSpace(capability)
	if required == "" {
		return fmt.Errorf("extension: capability is required")
	}

	grant := c.lookup(extName)
	if capabilityGranted(grant.security, required) {
		return nil
	}
	return newCapabilityDeniedError(required, []string{required}, grant.security)
}

// CheckHostAPI reports whether extName may call the Host API method under both
// the granted_actions and granted_security gates.
func (c *CapabilityChecker) CheckHostAPI(extName, method string) error {
	method = strings.TrimSpace(method)
	if method == "" {
		return fmt.Errorf("extension: host api method is required")
	}

	requiredSecurity, ok := hostAPIMethodSecurityCapability[method]
	if !ok {
		return fmt.Errorf("extension: unknown host api method %q", method)
	}

	grant := c.lookup(extName)
	if !slices.Contains(grant.actions, method) {
		return newCapabilityDeniedError(method, []string{method}, grant.actions)
	}
	if strings.TrimSpace(requiredSecurity) == "" {
		return nil
	}
	if !capabilityGranted(grant.security, requiredSecurity) {
		return newCapabilityDeniedError(method, []string{requiredSecurity}, grant.security)
	}
	return nil
}
