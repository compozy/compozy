package memories

import (
	"github.com/compozy/compozy/cli/cmd/resource"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	return resource.NewCommand(&resource.CommandConfig{
		Use:              "memories",
		Short:            "Manage memory import/export",
		Long:             "Import and export memory configurations from the project directory",
		ExportPath:       "/memories/export",
		ImportPath:       "/memories/import",
		SupportsStrategy: true,
	})
}
