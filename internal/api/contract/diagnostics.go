package contract

import diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"

// DiagnosticItem is the canonical actionable-diagnostic wire shape.
type DiagnosticItem = diagnosticcontract.DiagnosticItem

// DiagnosticCodeSpec records the canonical owner metadata for one code.
type DiagnosticCodeSpec = diagnosticcontract.DiagnosticCodeSpec

const (
	SeverityOK       = diagnosticcontract.SeverityOK
	SeverityInfo     = diagnosticcontract.SeverityInfo
	SeverityWarn     = diagnosticcontract.SeverityWarn
	SeverityError    = diagnosticcontract.SeverityError
	SeverityCritical = diagnosticcontract.SeverityCritical
)

const (
	FreshnessLive    = diagnosticcontract.FreshnessLive
	FreshnessOffline = diagnosticcontract.FreshnessOffline
	FreshnessStale   = diagnosticcontract.FreshnessStale
)

const (
	CategoryProvider   = diagnosticcontract.CategoryProvider
	CategoryDaemon     = diagnosticcontract.CategoryDaemon
	CategoryConfig     = diagnosticcontract.CategoryConfig
	CategoryVault      = diagnosticcontract.CategoryVault
	CategoryMCP        = diagnosticcontract.CategoryMCP
	CategoryBridge     = diagnosticcontract.CategoryBridge
	CategoryExtension  = diagnosticcontract.CategoryExtension
	CategorySession    = diagnosticcontract.CategorySession
	CategoryTask       = diagnosticcontract.CategoryTask
	CategoryHome       = diagnosticcontract.CategoryHome
	CategorySecrets    = diagnosticcontract.CategorySecrets
	CategoryMigrations = diagnosticcontract.CategoryMigrations
	CategoryNetwork    = diagnosticcontract.CategoryNetwork
)

const (
	CodeAgentNameReserved             = diagnosticcontract.CodeAgentNameReserved
	CodeBinaryVersionMismatch         = diagnosticcontract.CodeBinaryVersionMismatch
	CodeBridgeHealthUnavailable       = diagnosticcontract.CodeBridgeHealthUnavailable
	CodeBridgeNotFound                = diagnosticcontract.CodeBridgeNotFound
	CodeBridgeNotificationSuppressed  = diagnosticcontract.CodeBridgeNotificationSuppressed
	CodeBridgeReady                   = diagnosticcontract.CodeBridgeReady
	CodeBridgeTargetUnavailable       = diagnosticcontract.CodeBridgeTargetUnavailable
	CodeBulkTooLarge                  = diagnosticcontract.CodeBulkTooLarge
	CodeBundleConsentRequired         = diagnosticcontract.CodeBundleConsentRequired
	CodeBundlePartialFailure          = diagnosticcontract.CodeBundlePartialFailure
	CodeBundleSizeExceeded            = diagnosticcontract.CodeBundleSizeExceeded
	CodeConfigActiveSessionsBlock     = diagnosticcontract.CodeConfigActiveSessionsBlock
	CodeConfigApplyUnsupported        = diagnosticcontract.CodeConfigApplyUnsupported
	CodeConfigDriftPresent            = diagnosticcontract.CodeConfigDriftPresent
	CodeConfigDriftStale              = diagnosticcontract.CodeConfigDriftStale
	CodeConfigInvalid                 = diagnosticcontract.CodeConfigInvalid
	CodeConfigPartialFailure          = diagnosticcontract.CodeConfigPartialFailure
	CodeConfigReloadTimeout           = diagnosticcontract.CodeConfigReloadTimeout
	CodeConfigRestartRequired         = diagnosticcontract.CodeConfigRestartRequired
	CodeConfigValidateFailed          = diagnosticcontract.CodeConfigValidateFailed
	CodeConfigValidated               = diagnosticcontract.CodeConfigValidated
	CodeCursorConflict                = diagnosticcontract.CodeCursorConflict
	CodeDaemonHealthUnavailable       = diagnosticcontract.CodeDaemonHealthUnavailable
	CodeDaemonDraining                = diagnosticcontract.CodeDaemonDraining
	CodeDaemonStateSuspect            = diagnosticcontract.CodeDaemonStateSuspect
	CodeDaemonStatusOK                = diagnosticcontract.CodeDaemonStatusOK
	CodeDaemonUnavailable             = diagnosticcontract.CodeDaemonUnavailable
	CodeDiskWriteFailed               = diagnosticcontract.CodeDiskWriteFailed
	CodeExtensionBlockedByBundle      = diagnosticcontract.CodeExtensionBlockedByBundle
	CodeExtensionChecksumUnverified   = diagnosticcontract.CodeExtensionChecksumUnverified
	CodeExtensionInstallFailed        = diagnosticcontract.CodeExtensionInstallFailed
	CodeExtensionUpdateCleanupFailed  = diagnosticcontract.CodeExtensionUpdateCleanupFailed
	CodeExtensionInUse                = diagnosticcontract.CodeExtensionInUse
	CodeExtensionNotFound             = diagnosticcontract.CodeExtensionNotFound
	CodeExtensionRuntimeUnavailable   = diagnosticcontract.CodeExtensionRuntimeUnavailable
	CodeFlagNotApplicable             = diagnosticcontract.CodeFlagNotApplicable
	CodeForbiddenOperatorAction       = diagnosticcontract.CodeForbiddenOperatorAction
	CodeForceOpRateLimited            = diagnosticcontract.CodeForceOpRateLimited
	CodeForceOpRequiresReason         = diagnosticcontract.CodeForceOpRequiresReason
	CodeHomeDiskSpaceCritical         = diagnosticcontract.CodeHomeDiskSpaceCritical
	CodeHomeDiskSpaceLow              = diagnosticcontract.CodeHomeDiskSpaceLow
	CodeHomePathMissing               = diagnosticcontract.CodeHomePathMissing
	CodeHomePermsWrong                = diagnosticcontract.CodeHomePermsWrong
	CodeIDFormatUnknown               = diagnosticcontract.CodeIDFormatUnknown
	CodeIdentityLookupUnavailable     = diagnosticcontract.CodeIdentityLookupUnavailable
	CodeIdentityMismatch              = diagnosticcontract.CodeIdentityMismatch
	CodeIdentityRequired              = diagnosticcontract.CodeIdentityRequired
	CodeIdentityStale                 = diagnosticcontract.CodeIdentityStale
	CodeIdentityUnauthorized          = diagnosticcontract.CodeIdentityUnauthorized
	CodeMarketplaceUnavailable        = diagnosticcontract.CodeMarketplaceUnavailable
	CodeModelNotFound                 = diagnosticcontract.CodeModelNotFound
	CodeModelUnavailable              = diagnosticcontract.CodeModelUnavailable
	CodeMCPAuthRequired               = diagnosticcontract.CodeMCPAuthRequired
	CodeMCPServerReady                = diagnosticcontract.CodeMCPServerReady
	CodeMCPServerUnavailable          = diagnosticcontract.CodeMCPServerUnavailable
	CodeMigrationsPending             = diagnosticcontract.CodeMigrationsPending
	CodeNetworkDisabled               = diagnosticcontract.CodeNetworkDisabled
	CodeNetworkReady                  = diagnosticcontract.CodeNetworkReady
	CodeNetworkUnavailable            = diagnosticcontract.CodeNetworkUnavailable
	CodePresetBuiltinProtected        = diagnosticcontract.CodePresetBuiltinProtected
	CodePresetDuplicateName           = diagnosticcontract.CodePresetDuplicateName
	CodePresetFilterInvalid           = diagnosticcontract.CodePresetFilterInvalid
	CodePresetNotFound                = diagnosticcontract.CodePresetNotFound
	CodeProbeFailed                   = diagnosticcontract.CodeProbeFailed
	CodeProbeTimeout                  = diagnosticcontract.CodeProbeTimeout
	CodeProviderClassificationUnknown = diagnosticcontract.CodeProviderClassificationUnknown
	CodeProviderCLIMissing            = diagnosticcontract.CodeProviderCLIMissing
	CodeProviderCredentialUnresolved  = diagnosticcontract.CodeProviderCredentialUnresolved
	CodeProviderLoginRequiresLocalTTY = diagnosticcontract.CodeProviderLoginRequiresLocalTTY
	CodeProviderAuthenticated         = diagnosticcontract.CodeProviderAuthenticated
	CodeProviderNotAuthenticated      = diagnosticcontract.CodeProviderNotAuthenticated
	CodeProviderNotInstalled          = diagnosticcontract.CodeProviderNotInstalled
	CodeProviderPermissionDenied      = diagnosticcontract.CodeProviderPermissionDenied
	CodeProviderRateLimited           = diagnosticcontract.CodeProviderRateLimited
	CodeProviderTransientFailure      = diagnosticcontract.CodeProviderTransientFailure
	CodeReasoningEffortUnsupported    = diagnosticcontract.CodeReasoningEffortUnsupported
	CodeReasoningOptionMissing        = diagnosticcontract.CodeReasoningOptionMissing
	CodeRoleAgentNotFound             = diagnosticcontract.CodeRoleAgentNotFound
	CodeRoleResolutionFailed          = diagnosticcontract.CodeRoleResolutionFailed
	CodeRoleUnknown                   = diagnosticcontract.CodeRoleUnknown
	CodeRetryChainTooDeep             = diagnosticcontract.CodeRetryChainTooDeep
	CodeSchedulerReady                = diagnosticcontract.CodeSchedulerReady
	CodeSchedulerPaused               = diagnosticcontract.CodeSchedulerPaused
	CodeSecretsPermsWrong             = diagnosticcontract.CodeSecretsPermsWrong
	CodeSessionBusy                   = diagnosticcontract.CodeSessionBusy
	CodeSessionLocked                 = diagnosticcontract.CodeSessionLocked
	CodeSessionQueueFull              = diagnosticcontract.CodeSessionQueueFull
	CodeSessionResumeAmbiguous        = diagnosticcontract.CodeSessionResumeAmbiguous
	CodeSkillRegistryReady            = diagnosticcontract.CodeSkillRegistryReady
	CodeSkillNotFound                 = diagnosticcontract.CodeSkillNotFound
	CodeSocketPathUnwritable          = diagnosticcontract.CodeSocketPathUnwritable
	CodeTargetAmbiguous               = diagnosticcontract.CodeTargetAmbiguous
	CodeTargetUnknown                 = diagnosticcontract.CodeTargetUnknown
	CodeTaskRunAlreadyTerminal        = diagnosticcontract.CodeTaskRunAlreadyTerminal
	CodeTaskRunCrashed                = diagnosticcontract.CodeTaskRunCrashed
	CodeTaskRunNotReleasable          = diagnosticcontract.CodeTaskRunNotReleasable
	CodeTaskRunOrphan                 = diagnosticcontract.CodeTaskRunOrphan
	CodeTaskRunStaleLease             = diagnosticcontract.CodeTaskRunStaleLease
	CodeTaskRunStillActive            = diagnosticcontract.CodeTaskRunStillActive
	CodeTaskRunStranded               = diagnosticcontract.CodeTaskRunStranded
	CodeTaskRunStuck                  = diagnosticcontract.CodeTaskRunStuck
	CodeUnknownActorFormat            = diagnosticcontract.CodeUnknownActorFormat
	CodeUnknownComponent              = diagnosticcontract.CodeUnknownComponent
	CodeVaultRefUnresolved            = diagnosticcontract.CodeVaultRefUnresolved
)

const (
	CodeExtensionArchiveDigestMismatch   = diagnosticcontract.CodeExtensionArchiveDigestMismatch
	CodeExtensionUnverifiedPolicyBlocked = diagnosticcontract.CodeExtensionUnverifiedPolicyBlocked
)

// DiagnosticCodeSpecs returns the sorted canonical diagnostic code registry.
func DiagnosticCodeSpecs() []DiagnosticCodeSpec {
	return diagnosticcontract.DiagnosticCodeSpecs()
}

// DiagnosticCodes returns all registered deterministic diagnostic codes.
func DiagnosticCodes() []string {
	return diagnosticcontract.DiagnosticCodes()
}

// IsDiagnosticCode reports whether code belongs to the canonical registry.
func IsDiagnosticCode(code string) bool {
	return diagnosticcontract.IsDiagnosticCode(code)
}

// DiagnosticCodeCategory returns the canonical category for code.
func DiagnosticCodeCategory(code string) (string, bool) {
	return diagnosticcontract.DiagnosticCodeCategory(code)
}

// IsDiagnosticSeverity reports whether severity belongs to the closed set.
func IsDiagnosticSeverity(severity string) bool {
	return diagnosticcontract.IsDiagnosticSeverity(severity)
}

// IsDiagnosticFreshness reports whether freshness belongs to the closed set.
func IsDiagnosticFreshness(freshness string) bool {
	return diagnosticcontract.IsDiagnosticFreshness(freshness)
}

// IsDiagnosticCategory reports whether category belongs to the closed set.
func IsDiagnosticCategory(category string) bool {
	return diagnosticcontract.IsDiagnosticCategory(category)
}

// ValidateDiagnosticItem checks the public contract without mutating it.
func ValidateDiagnosticItem(item DiagnosticItem) error {
	return diagnosticcontract.ValidateDiagnosticItem(item)
}

// ValidateDiagnosticRegistry checks duplicate codes and unknown categories.
func ValidateDiagnosticRegistry() error {
	return diagnosticcontract.ValidateDiagnosticRegistry()
}
