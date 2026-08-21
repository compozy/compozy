package corecmds

import "github.com/compozy/compozy/internal/cmdpalette"

func profileCommands() []cmdpalette.Descriptor {
	return []cmdpalette.Descriptor{
		profileUseCommand(),
		profileFlowCommand("profile.create", "Create profile", "create", "plus", []cmdpalette.Argument{
			profileTextArgument("name", "Profile name"),
		}),
		profileFlowCommand("profile.update", "Update profile", "update", "pencil", []cmdpalette.Argument{
			profileTextArgument("profile", "Profile name"),
		}),
		profileFlowCommand("profile.rename", "Rename profile", "rename", "text-cursor-input", []cmdpalette.Argument{
			profileTextArgument("profile", "Current profile name"),
			profileTextArgument("new_name", "New profile name"),
		}),
		profileFlowCommand("profile.archive", "Archive profile", "archive", "archive", []cmdpalette.Argument{
			profileTextArgument("profile", "Profile name"),
		}),
		profileFlowCommand(
			"profile.unarchive",
			"Unarchive profile",
			"unarchive",
			"archive-restore",
			[]cmdpalette.Argument{
				profileTextArgument("profile", "Profile name"),
			},
		),
		profileDeleteCommand(),
	}
}

func profileUseCommand() cmdpalette.Descriptor {
	command := clientCommand("profile.use", "Use profile", coreSectionProfiles, "user-round-check")
	command.Arguments = []cmdpalette.Argument{profileTextArgument("profile", "Profile name")}
	command.Keywords = []string{"switch", "active", "selection"}
	return command
}

func profileFlowCommand(
	id cmdpalette.CommandID,
	title, flow, icon string,
	arguments []cmdpalette.Argument,
) cmdpalette.Descriptor {
	command := coreDescriptor(id, title, coreSectionProfiles, icon, cmdpalette.Action{
		Kind: cmdpalette.ActionKindNavigate,
		App:  coreSettingsKey,
		Args: map[string]any{"pathname": "/settings/profiles", "flow": flow},
	})
	command.Arguments = arguments
	return command
}

func profileDeleteCommand() cmdpalette.Descriptor {
	command := profileFlowCommand(
		"profile.delete", "Delete profile", "delete", "trash-2",
		[]cmdpalette.Argument{profileTextArgument("profile", "Profile name")},
	)
	command.Destructive = true
	command.Confirmation = &cmdpalette.Confirmation{
		Title:   "Delete profile?",
		Body:    "The canonical delete plan must confirm that the profile owns no work.",
		Confirm: "Delete",
	}
	return command
}

func profileTextArgument(name, placeholder string) cmdpalette.Argument {
	return cmdpalette.Argument{Name: name, Type: cmdpalette.ArgumentTypeText, Placeholder: placeholder, Required: true}
}
