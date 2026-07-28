package extensionpkg

import "context"

// ExtensionToolCallTracker binds reverse Host API calls to one active tool invocation.
type ExtensionToolCallTracker interface {
	BeginExtensionToolCall(
		ctx context.Context,
		extensionName string,
		sessionID string,
	) (string, func(), error)
}

// WithExtensionToolCallTracker installs active-call correlation for extension-host tools.
func WithExtensionToolCallTracker(tracker ExtensionToolCallTracker) Option {
	return func(manager *Manager) {
		manager.toolCallTracker = tracker
	}
}
