package corecmds

import "github.com/compozy/compozy/internal/cmdpalette"

type automationCreateDefinition struct {
	start    string
	title    string
	icon     string
	keywords []string
}

// automationCreateDefinitions open the Automations editor with Starts
// preselected; the web route reads `create=1&start=…` as the editor deep link.
var automationCreateDefinitions = []automationCreateDefinition{
	{
		start:    coreStartSchedule,
		title:    "New scheduled automation",
		icon:     coreIconPlus,
		keywords: []string{"job", coreStartSchedule, "cron", "every"},
	},
	{
		start:    coreStartEvent,
		title:    "New automation on an event",
		icon:     coreIconPlus,
		keywords: []string{"trigger", coreStartEvent, "when", coreKeywordWebhook},
	},
}

func automationCommands() []cmdpalette.Descriptor {
	commands := make([]cmdpalette.Descriptor, 0, len(automationCreateDefinitions))
	for _, definition := range automationCreateDefinitions {
		command := coreDescriptor(
			cmdpalette.CommandID("automation.create."+definition.start),
			definition.title,
			coreSectionApps,
			definition.icon,
			cmdpalette.Action{
				Kind: cmdpalette.ActionKindNavigate,
				App:  coreAppAutomations,
				Args: map[string]any{
					coreArgPathname: "/automations",
					"create":        "1",
					"start":         definition.start,
				},
			},
		)
		command.Keywords = append([]string(nil), definition.keywords...)
		commands = append(commands, command)
	}
	return commands
}
