package schemas

import (
	"github.com/compozy/compozy/cli/cmd/resource"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	return resource.NewCommand(&resource.CommandConfig{
		Use:              "schemas",
		Short:            "Manage schema import/export",
		Long:             "Import and export schema definitions from the project directory",
		ExportPath:       "/schemas/export",
		ImportPath:       "/schemas/import",
		SupportsStrategy: true,
	})
}
