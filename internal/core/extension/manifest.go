package extensions

import "time"

const (
	// ManifestFileNameTOML is the preferred manifest filename.
	ManifestFileNameTOML = "extension.toml"
	// ManifestFileNameJSON is the JSON fallback manifest filename.
	ManifestFileNameJSON = "extension.json"
	// DefaultHookPriority is the default priority for hook declarations.
	DefaultHookPriority = 500
	// MinHookPriority is the lowest allowed hook priority.
	MinHookPriority = 0
	// MaxHookPriority is the highest allowed hook priority.
	MaxHookPriority = 1000
)

// Source identifies where an extension was discovered.
type Source string

const (
	// SourceBundled identifies bundled extensions embedded in the Compozy binary.
	SourceBundled Source = "bundled"
	// SourceUser identifies user-scoped extensions installed under the user's home directory.
	SourceUser Source = "user"
	// SourceWorkspace identifies workspace-scoped extensions stored in the repository.
	SourceWorkspace Source = "workspace"
)

// Capability declares a manifest capability grant.
type Capability string

// Capability values define the supported extension capability taxonomy.
const (
	CapabilityEventsRead        Capability = "events.read"
	CapabilityEventsPublish     Capability = "events.publish"
	CapabilityPromptMutate      Capability = "prompt.mutate"
	CapabilityPlanMutate        Capability = "plan.mutate"
	CapabilityAgentMutate       Capability = "agent.mutate"
	CapabilityJobMutate         Capability = "job.mutate"
	CapabilityRunMutate         Capability = "run.mutate"
	CapabilityReviewMutate      Capability = "review.mutate"
	CapabilityArtifactsRead     Capability = "artifacts.read"
	CapabilityArtifactsWrite    Capability = "artifacts.write"
	CapabilityTasksRead         Capability = "tasks.read"
	CapabilityTasksCreate       Capability = "tasks.create"
	CapabilityRunsStart         Capability = "runs.start"
	CapabilityMemoryRead        Capability = "memory.read"
	CapabilityMemoryWrite       Capability = "memory.write"
	CapabilityProvidersRegister Capability = "providers.register"
	CapabilitySkillsShip        Capability = "skills.ship"
	CapabilitySubprocessSpawn   Capability = "subprocess.spawn"
	CapabilityNetworkEgress     Capability = "network.egress"
)

// HookName identifies a canonical extension hook event.
type HookName string

// Hook names define the supported extension hook taxonomy.
const (
	HookPlanPreDiscover        HookName = "plan.pre_discover"
	HookPlanPostDiscover       HookName = "plan.post_discover"
	HookPlanPreGroup           HookName = "plan.pre_group"
	HookPlanPostGroup          HookName = "plan.post_group"
	HookPlanPrePrepareJobs     HookName = "plan.pre_prepare_jobs"
	HookPlanPostPrepareJobs    HookName = "plan.post_prepare_jobs"
	HookPromptPreBuild         HookName = "prompt.pre_build"
	HookPromptPostBuild        HookName = "prompt.post_build"
	HookPromptPreSystem        HookName = "prompt.pre_system"
	HookAgentPreSessionCreate  HookName = "agent.pre_session_create"
	HookAgentPostSessionCreate HookName = "agent.post_session_create"
	HookAgentPreSessionResume  HookName = "agent.pre_session_resume"
	HookAgentOnSessionUpdate   HookName = "agent.on_session_update"
	HookAgentPostSessionEnd    HookName = "agent.post_session_end"
	HookJobPreExecute          HookName = "job.pre_execute"
	HookJobPostExecute         HookName = "job.post_execute"
	HookJobPreRetry            HookName = "job.pre_retry"
	HookRunPreStart            HookName = "run.pre_start"
	HookRunPostStart           HookName = "run.post_start"
	HookRunPreShutdown         HookName = "run.pre_shutdown"
	HookRunPostShutdown        HookName = "run.post_shutdown"
	HookReviewPreFetch         HookName = "review.pre_fetch"
	HookReviewPostFetch        HookName = "review.post_fetch"
	HookReviewPreBatch         HookName = "review.pre_batch"
	HookReviewPostFix          HookName = "review.post_fix"
	HookReviewPreResolve       HookName = "review.pre_resolve"
	HookArtifactPreWrite       HookName = "artifact.pre_write"
	HookArtifactPostWrite      HookName = "artifact.post_write"
)

// Manifest is the parsed extension manifest shared by discovery and runtime tasks.
type Manifest struct {
	Extension  ExtensionInfo     `toml:"extension"  json:"extension"`
	Subprocess *SubprocessConfig `toml:"subprocess" json:"subprocess,omitempty"`
	Security   SecurityConfig    `toml:"security"   json:"security"`
	Hooks      []HookDeclaration `toml:"hooks"      json:"hooks,omitempty"`
	Resources  ResourcesConfig   `toml:"resources"  json:"resources,omitempty"`
	Providers  ProvidersConfig   `toml:"providers"  json:"providers,omitempty"`
}

// ExtensionInfo contains the identifying metadata for one extension.
type ExtensionInfo struct {
	Name              string `toml:"name"                json:"name"`
	Version           string `toml:"version"             json:"version"`
	Description       string `toml:"description"         json:"description"`
	MinCompozyVersion string `toml:"min_compozy_version" json:"min_compozy_version"`
}

// SubprocessConfig configures the extension subprocess entrypoint.
type SubprocessConfig struct {
	Command           string            `toml:"command"             json:"command"`
	Args              []string          `toml:"args"                json:"args,omitempty"`
	Env               map[string]string `toml:"env"                 json:"env,omitempty"`
	ShutdownTimeout   time.Duration     `toml:"shutdown_timeout"    json:"shutdown_timeout,omitempty"`
	HealthCheckPeriod time.Duration     `toml:"health_check_period" json:"health_check_period,omitempty"`
}

// SecurityConfig declares the capabilities requested by an extension.
type SecurityConfig struct {
	Capabilities []Capability `toml:"capabilities" json:"capabilities"`
}

// HookDeclaration declares one hook subscription exposed by an extension.
type HookDeclaration struct {
	Event    HookName      `toml:"event"    json:"event"`
	Priority int           `toml:"priority" json:"priority,omitempty"`
	Required bool          `toml:"required" json:"required,omitempty"`
	Timeout  time.Duration `toml:"timeout"  json:"timeout,omitempty"`
}

// ResourcesConfig declares declarative assets shipped with an extension.
type ResourcesConfig struct {
	Skills []string `toml:"skills" json:"skills,omitempty"`
}

// ProvidersConfig declares provider overlays exported by an extension.
type ProvidersConfig struct {
	IDE    []ProviderEntry `toml:"ide"    json:"ide,omitempty"`
	Review []ProviderEntry `toml:"review" json:"review,omitempty"`
	Model  []ProviderEntry `toml:"model"  json:"model,omitempty"`
}

// ProviderEntry declares one provider overlay entry from a manifest.
type ProviderEntry struct {
	Name               string             `toml:"name"                 json:"name"`
	Kind               ProviderKind       `toml:"kind"                 json:"kind,omitempty"`
	Target             string             `toml:"target"               json:"target,omitempty"`
	Command            string             `toml:"command"              json:"command,omitempty"`
	DisplayName        string             `toml:"display_name"         json:"display_name,omitempty"`
	SetupAgentName     string             `toml:"setup_agent_name"     json:"setup_agent_name,omitempty"`
	DefaultModel       string             `toml:"default_model"        json:"default_model,omitempty"`
	SupportsAddDirs    *bool              `toml:"supports_add_dirs"    json:"supports_add_dirs,omitempty"`
	UsesBootstrapModel *bool              `toml:"uses_bootstrap_model" json:"uses_bootstrap_model,omitempty"`
	DocsURL            string             `toml:"docs_url"             json:"docs_url,omitempty"`
	InstallHint        string             `toml:"install_hint"         json:"install_hint,omitempty"`
	FullAccessModeID   string             `toml:"full_access_mode_id"  json:"full_access_mode_id,omitempty"`
	FixedArgs          []string           `toml:"fixed_args"           json:"fixed_args,omitempty"`
	ProbeArgs          []string           `toml:"probe_args"           json:"probe_args,omitempty"`
	Env                map[string]string  `toml:"env"                  json:"env,omitempty"`
	Fallbacks          []ProviderLauncher `toml:"fallbacks"            json:"fallbacks,omitempty"`
	Bootstrap          *ProviderBootstrap `toml:"bootstrap"            json:"bootstrap,omitempty"`
	Metadata           map[string]string  `toml:"metadata"             json:"metadata,omitempty"`
}

type capabilitySet map[Capability]struct{}

func newCapabilitySet(values ...Capability) capabilitySet {
	set := make(capabilitySet, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func (s capabilitySet) contains(value Capability) bool {
	_, ok := s[value]
	return ok
}

type hookNameSet map[HookName]struct{}

func newHookNameSet(values ...HookName) hookNameSet {
	set := make(hookNameSet, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func (s hookNameSet) contains(value HookName) bool {
	_, ok := s[value]
	return ok
}

var supportedCapabilities = newCapabilitySet(
	CapabilityEventsRead,
	CapabilityEventsPublish,
	CapabilityPromptMutate,
	CapabilityPlanMutate,
	CapabilityAgentMutate,
	CapabilityJobMutate,
	CapabilityRunMutate,
	CapabilityReviewMutate,
	CapabilityArtifactsRead,
	CapabilityArtifactsWrite,
	CapabilityTasksRead,
	CapabilityTasksCreate,
	CapabilityRunsStart,
	CapabilityMemoryRead,
	CapabilityMemoryWrite,
	CapabilityProvidersRegister,
	CapabilitySkillsShip,
	CapabilitySubprocessSpawn,
	CapabilityNetworkEgress,
)

var supportedHookNames = newHookNameSet(
	HookPlanPreDiscover,
	HookPlanPostDiscover,
	HookPlanPreGroup,
	HookPlanPostGroup,
	HookPlanPrePrepareJobs,
	HookPlanPostPrepareJobs,
	HookPromptPreBuild,
	HookPromptPostBuild,
	HookPromptPreSystem,
	HookAgentPreSessionCreate,
	HookAgentPostSessionCreate,
	HookAgentPreSessionResume,
	HookAgentOnSessionUpdate,
	HookAgentPostSessionEnd,
	HookJobPreExecute,
	HookJobPostExecute,
	HookJobPreRetry,
	HookRunPreStart,
	HookRunPostStart,
	HookRunPreShutdown,
	HookRunPostShutdown,
	HookReviewPreFetch,
	HookReviewPostFetch,
	HookReviewPreBatch,
	HookReviewPostFix,
	HookReviewPreResolve,
	HookArtifactPreWrite,
	HookArtifactPostWrite,
)
