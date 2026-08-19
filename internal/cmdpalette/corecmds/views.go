package corecmds

import "github.com/compozy/compozy/internal/cmdpalette"

func viewCommands() []cmdpalette.Descriptor {
	return []cmdpalette.Descriptor{
		coreDescriptor(
			"palette.view.sessions",
			"Sessions",
			"Views",
			"square-terminal",
			cmdpalette.Action{Kind: cmdpalette.ActionKindView, View: "sessions"},
		),
	}
}
