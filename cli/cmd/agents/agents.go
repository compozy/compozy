package agents

import (
	"github.com/compozy/compozy/cli/cmd/resource"
	"github.com/spf13/cobra"
)

const (
	exportPath = "/agents/export"
	importPath = "/agents/import"
)

func Cmd() *cobra.Command {
	return resource.NewCommand(&resource.CommandConfig{
		Use:              "agents",
		Short:            "Manage agent import/export",
		Long:             "Import and export agent configurations from the project directory",
		ExportPath:       exportPath,
		ImportPath:       importPath,
		SupportsStrategy: true,
	})
}
