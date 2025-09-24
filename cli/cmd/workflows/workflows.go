package workflows

import (
	"github.com/compozy/compozy/cli/cmd/resource"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	return resource.NewCommand(&resource.CommandConfig{
		Use:              "workflows",
		Short:            "Manage workflow import/export",
		Long:             "Import and export workflow definitions from the project directory",
		ExportPath:       "/workflows/export",
		ImportPath:       "/workflows/import",
		SupportsStrategy: true,
	})
}
