package events

const (
	ExtensionInstallCompleted = "extension.install.completed"
	ExtensionInstallFailed    = "extension.install.failed"
	ExtensionUpdateCompleted  = "extension.update.completed"
	ExtensionUpdateFailed     = "extension.update.failed"
	ExtensionRemoved          = "extension.removed"
	ExtensionEnabled          = "extension.enabled"
	ExtensionDisabled         = "extension.disabled"
	ExtensionDigestVerify     = "extension.digest.verify"
	ExtensionDevLinked        = "extension.dev.linked"
	ExtensionDevUnlinked      = "extension.dev.unlinked"
	ExtensionReloadCompleted  = "extension.reload.completed"
	ExtensionReloadFailed     = "extension.reload.failed"
	ExtensionPublishCompleted = "extension.publish.completed"
	ExtensionPublishFailed    = "extension.publish.failed"
	ExtensionCrashLoopBackoff = "extension.crash_loop.backoff"
)
