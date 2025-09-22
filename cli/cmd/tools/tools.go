package tools

import (
	"github.com/compozy/compozy/cli/cmd/resource"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	return resource.NewCommand(&resource.CommandConfig{
		Use:              "tools",
		Short:            "Manage tool import/export",
		Long:             "Import and export tool definitions from the project directory",
		ExportPath:       "/tools/export",
		ImportPath:       "/tools/import",
		SupportsStrategy: true,
	})
}
