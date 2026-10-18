package windowmanager

import (
	"context"
	"strings"
)

type AppDeprecationObserver func(ctx context.Context, app, replacement, source string)

// WithAppDeprecationObserver observes public aliases until removal in v0.5.0.
func WithAppDeprecationObserver(observer AppDeprecationObserver) Option {
	return func(options *managerOptions) error {
		options.appDeprecationObserver = observer
		return nil
	}
}

// Public command aliases are removed in v0.5.0; stored-document migration remains.
func (m *Manager) canonicalCommandApps(ctx context.Context, request CommandRequest) CommandRequest {
	source := "command"
	if request.Actor.Kind == "cli" {
		source = "cli"
	}
	switch command := request.Payload.(type) {
	case OpenWindowCommand:
		command.Window.App = strings.TrimSpace(command.Window.App)
		if replacement, retired := RetiredApp(command.Window.App); retired {
			if m.appDeprecationObserver != nil {
				m.appDeprecationObserver(ctx, command.Window.App, replacement, source)
			}
			command.Window = command.Window.canonicalApp()
			request.Payload = command
		}
	case ReplaceLayoutCommand:
		command.Document = canonicalLayoutApps(ctx, command.Document, m.appDeprecationObserver, source)
		request.Payload = command
	}
	return request
}

// V4 exports migrate permanently; aliases on current-version public inputs end in v0.5.0.
func canonicalLayoutApps(
	ctx context.Context, document LayoutDocument, observer AppDeprecationObserver, source string,
) LayoutDocument {
	document = cloneLayoutDocument(document)
	if document.Version == PreviousSnapshotVersion {
		document.Version = SnapshotVersion
	}
	for id, window := range document.Windows {
		if replacement, retired := RetiredApp(window.App); retired {
			if observer != nil {
				observer(ctx, window.App, replacement, source)
			}
			document.Windows[id] = rewriteRetiredWindow(window)
		}
	}
	return document
}
