package mcps

import (
	"github.com/compozy/compozy/cli/cmd/resource"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	return resource.NewCommand(&resource.CommandConfig{
		Use:              "mcps",
		Short:            "Manage MCP import/export",
		Long:             "Import and export Model Context Protocol configurations from the project directory",
		ExportPath:       "/mcps/export",
		ImportPath:       "/mcps/import",
		SupportsStrategy: true,
	})
}
