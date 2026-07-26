package spec

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	httpMethodDelete = "DELETE"
	httpMethodGet    = "GET"
	httpMethodPatch  = "PATCH"
	httpMethodPost   = "POST"
	httpMethodPut    = "PUT"
)

const (
	specContentTypeEventStream = "text/event-stream"
	specFormatInt64            = "int64"
)

const (
	specNetworkThreadNotFoundDescription = "Network thread not found"
)

const (
	specAPIAutomationJobsIDPath                              = "/api/automation/jobs/{id}"
	specAPIAutomationTriggersIDPath                          = "/api/automation/triggers/{id}"
	specAPIBundlesActivationsIDPath                          = "/api/bundles/activations/{id}"
	specAPIExtensionsPath                                    = "/api/extensions"
	specAPIExtensionsNamePath                                = "/api/extensions/{name}"
	specAPIExtensionsNameProvenancePath                      = specAPIExtensionsNamePath + "/provenance"
	specAPIExtensionsNameEnablePath                          = specAPIExtensionsNamePath + "/enable"
	specAPIExtensionsNameDisablePath                         = specAPIExtensionsNamePath + "/disable"
	specAPIMemoryFilenamePath                                = "/api/memory/{filename}"
	specAPINotificationsPresetsPath                          = "/api/notifications/presets"
	specAPINotificationsPresetsNamePath                      = specAPINotificationsPresetsPath + "/{name}"
	specAPIResourcesKindIDPath                               = "/api/resources/{kind}/{id}"
	specAPISettingsAutomationPath                            = "/api/settings/automation"
	specAPISettingsGeneralPath                               = "/api/settings/general"
	specAPISettingsHooksExtensionsPath                       = "/api/settings/hooks-extensions"
	specAPISettingsHooksNamePath                             = "/api/settings/hooks/{name}"
	specAPISettingsMCPServersNamePath                        = "/api/settings/mcp-servers/{name}"
	specAPISettingsMemoryPath                                = "/api/settings/memory"
	specAPISettingsRolesPath                                 = "/api/settings/roles"
	specAPISettingsNetworkPath                               = "/api/settings/network"
	specAPISettingsWindowManagerPath                         = "/api/settings/window-manager"
	specAPISettingsObservabilityPath                         = "/api/settings/observability"
	specAPISettingsProvidersNamePath                         = "/api/settings/providers/{name}"
	specAPISettingsSandboxesNamePath                         = "/api/settings/sandboxes/{name}"
	specAPISettingsSkillsPath                                = "/api/settings/skills"
	specAPIRunsBulkFailPath                                  = "/api/runs/bulk/fail"
	specAPIRunsBulkReleasePath                               = "/api/runs/bulk/release"
	specAPIRunsIDFailPath                                    = "/api/runs/{id}/fail"
	specAPIRunsIDInspectPath                                 = "/api/runs/{id}/inspect"
	specAPIRunsIDReleasePath                                 = "/api/runs/{id}/release"
	specAPIRunsIDRetryPath                                   = "/api/runs/{id}/retry"
	specAPIRunsIDRecoverPath                                 = "/api/runs/{id}/recover"
	specAPISchedulerPath                                     = "/api/scheduler"
	specAPISchedulerBacklogPath                              = "/api/scheduler/backlog"
	specAPISchedulerDrainPath                                = "/api/scheduler/drain"
	specAPISchedulerPausePath                                = "/api/scheduler/pause"
	specAPISchedulerResumePath                               = "/api/scheduler/resume"
	specAPITaskRunsIDReviewsPath                             = "/api/task-runs/{id}/reviews"
	specAPITasksPath                                         = "/api/tasks"
	specAPITasksIDPath                                       = "/api/tasks/{id}"
	specAPITasksIDBlocksPath                                 = specAPITasksIDPath + "/blocks"
	specAPITasksIDBlocksBlockIDClearPath                     = specAPITasksIDBlocksPath + "/{block_id}/clear"
	specAPITasksIDExecutionProfilePath                       = "/api/tasks/{id}/execution-profile"
	specAPITasksIDInspectPath                                = "/api/tasks/{id}/inspect"
	specAPITasksIDNotificationsBridgesPath                   = "/api/tasks/{id}/notifications/bridges"
	specAPITasksIDNotificationsBridgesSubscriptionIDPath     = "/api/tasks/{id}/notifications/bridges/{subscription_id}"
	specAPITasksIDPausePath                                  = "/api/tasks/{id}/pause"
	specAPITasksIDRecoverPath                                = specAPITasksIDPath + "/recover"
	specAPITasksIDResumePath                                 = "/api/tasks/{id}/resume"
	specAPITasksIDRunsPath                                   = "/api/tasks/{id}/runs"
	specAPITasksIDRunsFanOutPath                             = specAPITasksIDRunsPath + "/fan-out"
	specAPIVaultEntriesPath                                  = "/api/vault/secrets"
	specAPIWorkspacesIDPath                                  = "/api/workspaces/{id}"
	specAcceptedDescription                                  = "Accepted"
	specActivationConflictDescription                        = "Activation conflict"
	specActivationNotFoundDescription                        = "Activation not found"
	specAgentCallerIdentityIsMissingDescription              = "Agent caller identity is missing"
	specAgentNotFoundDescription                             = "Agent not found"
	specAutomationJobNotFoundDescription                     = "Automation job not found"
	specAutomationManagerIsNotConfiguredDescription          = "Automation manager is not configured"
	specAutomationTriggerNotFoundDescription                 = "Automation trigger not found"
	specBridgeInstanceNotFoundDescription                    = "Bridge instance not found"
	specBridgeServiceIsNotConfiguredDescription              = "Bridge service is not configured"
	specBundleServiceIsNotConfiguredDescription              = "Bundle service is not configured"
	specConflictingSettingsChangeDescription                 = "Conflicting settings change"
	specCreatedDescription                                   = "Created"
	specExtensionNotFoundDescription                         = "Extension not found"
	specExtensionServiceIsNotConfiguredDescription           = "Extension service is not configured"
	specForbiddenDescription                                 = "Forbidden"
	specForceOperationForbiddenDescription                   = "Force operation forbidden"
	specForceOperationRateLimitExceededDescription           = "Force-operation rate limit exceeded"
	specSupportBundleServiceIsNotConfiguredDescription       = "Support bundle service is not configured"
	specForbiddenWorkspaceOrPermissionMismatchDescription    = "Forbidden - workspace or permission mismatch"
	specInternalDaemonErrorDescription                       = "Internal daemon error"
	specInternalServerErrorDescription                       = "Internal server error"
	specObserveNotConfiguredDescription                      = "Observe service is not configured"
	specOnboardingStoreUnavailableDescription                = "Onboarding store is not configured"
	specSessionPromptConflictDescription                     = "Session prompt conflict"
	specInvalidAgentLocalLayerDescription                    = "Invalid agent-local layer"
	specInvalidAutomationRunFilterDescription                = "Invalid automation run filter"
	specInvalidBridgeStateTransitionDescription              = "Invalid bridge state transition"
	specInvalidFilterDescription                             = "Invalid filter"
	specInvalidSettingsPayloadDescription                    = "Invalid settings payload"
	specInvalidSkillLookupDescription                        = "Invalid skill lookup"
	specInvalidTaskIDDescription                             = "Invalid task id"
	specInvalidTaskTriageRequestDescription                  = "Invalid task triage request"
	specNetworkRuntimeIsNotConfiguredDescription             = "Network runtime is not configured"
	specNotificationPresetNotFoundDescription                = "Notification preset not found"
	specNotificationPresetServiceIsNotConfiguredDescription  = "Notification preset service is not configured"
	specNoContentDescription                                 = "No Content"
	specPayloadTooLargeDescription                           = "Payload too large"
	specRateLimitedDescription                               = "Rate limited"
	specProviderNotFoundDescription                          = "Provider not found"
	specServiceUnavailableDependentServiceMissingDescription = "Service unavailable - dependent service missing"
	specSessionNotFoundDescription                           = "Session not found"
	specSkillMarketplaceIsNotConfiguredDescription           = "Skill marketplace is not configured"
	specSkillOrScopeNotFoundDescription                      = "Skill or scope not found"
	specSkillsRegistryIsNotConfiguredDescription             = "Skills registry is not configured"
	specTaskNotFoundDescription                              = "Task not found"
	specTaskAdmissionUnavailableDescription                  = "Task service is unavailable or daemon is draining"
	specTaskOrBridgeServiceIsNotConfiguredDescription        = "Task or bridge service is not configured"
	specTaskRunNotFoundDescription                           = "Task run not found"
	specTaskServiceIsNotConfiguredDescription                = "Task service is not configured"
	specToolNotFoundDescription                              = "Tool not found"
	specToolRegistryUnavailableDescription                   = "Tool registry unavailable"
	specVaultServiceUnavailableDescription                   = "Vault service unavailable"
	specWorkspaceNotFoundDescription                         = "Workspace not found"
	specWorkspaceRootMissingDescription                      = "Workspace root missing"
	specAgentDefinitionServicesUnavailableDescription        = "Workspace resolver or definition sync unavailable"
	specAPIAgentsNamePath                                    = "/api/agents/{name}"
	specAgentKey                                             = "agent"
	specAgentsKey                                            = "agents"
	specAutomationKey                                        = "automation"
	specBridgesKey                                           = "bridges"
	specBundlesKey                                           = "bundles"
	specDiagnosticsKey                                       = "diagnostics"
	specOnboardingKey                                        = "onboarding"
	specFilesystemKey                                        = "filesystem"
	specExtensionKey                                         = "extension"
	specExtensionsKey                                        = "extensions"
	specHooksKey                                             = "hooks"
	specIntegerKey                                           = "integer"
	specLogsKey                                              = "logs"
	specMarketplaceKey                                       = "marketplace"
	specMemoryKey                                            = "memory"
	specNetworkKey                                           = "network"
	specNotificationsKey                                     = "notifications"
	specObserveKey                                           = "observe"
	specProvidersKey                                         = "providers"
	specResourcesKey                                         = "resources"
	specRolesKey                                             = "roles"
	specSessionsKey                                          = "sessions"
	specSettingsKey                                          = "settings"
	specSkillsKey                                            = "skills"
	specSupportKey                                           = "support"
	specTasksKey                                             = "tasks"
	specToolsKey                                             = "tools"
	specToolsetsKey                                          = "toolsets"
	specVaultKey                                             = "vault"
	specWorkspacesKey                                        = "workspaces"
)

const (
	// DefaultPath is the canonical generated OpenAPI output location.
	DefaultPath = "openapi/agh.json"
)

var rawMessageType = reflect.TypeFor[json.RawMessage]()

// Transport identifies which daemon transport exposes a route.
type Transport string

const (
	TransportHTTP                   Transport = "http"
	TransportUDS                    Transport = "uds"
	workspaceRootMissingDescription           = "Workspace root is missing"
)

type binaryResponse struct{}

// ParameterSpec describes one OpenAPI parameter.
type ParameterSpec struct {
	Name        string
	In          string
	Description string
	Required    bool
	Kind        string
	Format      string
	Enum        []string
	Maximum     *float64
}

// ResponseSpec describes one OpenAPI response.
type ResponseSpec struct {
	Status      int
	Description string
	Body        any
	Bodies      []any
	ContentType string
}

// OperationSpec describes one canonical REST operation.
type OperationSpec struct {
	Method      string
	Path        string
	OperationID string
	Summary     string
	Tags        []string
	Transports  []Transport
	Parameters  []ParameterSpec
	RequestBody any
	// RequestBodyOptional keeps a request body schema documented while allowing empty requests.
	RequestBodyOptional bool
	Responses           []ResponseSpec
}

// Document builds the canonical OpenAPI specification document.
func Document() (*openapi3.T, error) {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:   "AGH API",
			Version: "1.0.0",
		},
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{},
		},
		Paths: openapi3.NewPaths(),
		Tags: openapi3.Tags{
			{Name: specAgentKey},
			{Name: specAgentsKey},
			{Name: specAutomationKey},
			{Name: specBridgesKey},
			{Name: specBundlesKey},
			{Name: specDiagnosticsKey},
			{Name: specOnboardingKey},
			{Name: specFilesystemKey},
			{Name: specNetworkKey},
			{Name: specNotificationsKey},
			{Name: specExtensionsKey},
			{Name: specHooksKey},
			{Name: specLogsKey},
			{Name: specLoopsKey},
			{Name: specMarketplaceKey},
			{Name: specMemoryKey},
			{Name: specObserveKey},
			{Name: specOpenAIKey},
			{Name: specProvidersKey},
			{Name: specResourcesKey},
			{Name: specRolesKey},
			{Name: specSessionsKey},
			{Name: specSettingsKey},
			{Name: specSupportKey},
			{Name: specSkillsKey},
			{Name: specTasksKey},
			{Name: specToolsKey},
			{Name: specToolsetsKey},
			{Name: specVaultKey},
			{Name: specWorkspacesKey},
		},
	}
	if err := registerWindowManagerComponentSchemas(doc.Components.Schemas); err != nil {
		return nil, fmt.Errorf("register window-manager component schemas: %w", err)
	}

	for _, opSpec := range Operations() {
		operation, err := buildOperation(doc.Components.Schemas, opSpec)
		if err != nil {
			return nil, fmt.Errorf("build %s %s: %w", opSpec.Method, opSpec.Path, err)
		}
		doc.AddOperation(opSpec.Path, opSpec.Method, operation)
	}
	resolveComponentSchemaReferences(doc.Components.Schemas)

	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validate openapi: %w", err)
	}

	return doc, nil
}
