package agents

import (
	"github.com/compozy/compozy/cli/cmd/resource"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	return resource.NewCommand(&resource.CommandConfig{
		Use:              "agents",
		Short:            "Manage agent import/export",
		Long:             "Import and export agent configurations from the project directory",
		ExportPath:       "/agents/export",
		ImportPath:       "/agents/import",
		SupportsStrategy: true,
	})
}
