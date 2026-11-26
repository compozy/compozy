package httpapi

import "github.com/compozy/compozy/internal/remoteproto"

func registerSignedOutOperatorRoutes(api operatorRouteGroups, handlers *Handlers) {
	status := api.group("/status", remoteproto.ScopeSessions)
	status.GET("", handlers.GetStatus)
	status.GET("/identity", handlers.GetRuntimeIdentity)

	settings := api.group("/settings", remoteproto.ScopeSettings)
	privileged := handlers.privilegedMutationGuard()
	settings.GET("/update", handlers.GetSettingsUpdate)
	settings.POST("/update/check", privileged, handlers.CheckSettingsUpdate)
	settings.POST("/update/apply", privileged, handlers.ApplySettingsUpdate)
	settings.POST("/update/cancel", privileged, handlers.CancelSettingsUpdate)
	settings.POST("/actions/restart", privileged, handlers.TriggerSettingsRestart)
	settings.GET("/actions/restart/:operation_id", handlers.GetSettingsRestartStatus)
}

func registerOperatorRoutes(api operatorRouteGroups, handlers *Handlers) {
	registerStatusRoutes(api, handlers)
	registerOnboardingRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerFilesystemRoutes(api.group("", remoteproto.ScopeFilesR), handlers)
	registerNotificationRoutes(api.group("", remoteproto.ScopeSessions), handlers)
	registerProfileRoutes(api, handlers)
	registerWorkspaceRoutes(api, handlers)
	registerWorktreeRoutes(api, handlers)
	registerWindowManagerRoutes(api.group("", remoteproto.ScopeSessions), handlers)
	registerTerminalRoutes(api.group("", remoteproto.ScopeTerminal), handlers)
	registerCmdPaletteRoutes(api, handlers)
	registerSessionRoutes(api, handlers)
	registerAgentManagementRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerRoleRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerLogsRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerSupportRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerObserveRoutes(api, handlers)
	registerHookRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerToolRoutes(api, handlers)
	registerAutomationRoutes(api.group("", remoteproto.ScopeSessions), handlers)
	registerLoopRoutes(api.group("", remoteproto.ScopeSessions), handlers)
	registerTaskRoutes(api.group("", remoteproto.ScopeTasks), handlers, true, api.remote)
	registerMarketplaceRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerSkillRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerExtensionRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerSettingsRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerVaultRoutes(api, handlers)
	registerProviderRoutes(api.group("", remoteproto.ScopeSessions), handlers)
	registerModelCatalogRoutes(api, handlers)
	registerProviderUsageRoutes(api.group("", remoteproto.ScopeSettings), handlers)
	registerOpenAIModelRoutes(api.group("", remoteproto.ScopeSessions), handlers)
}
