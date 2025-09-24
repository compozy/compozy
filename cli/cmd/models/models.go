package models

import (
	"github.com/compozy/compozy/cli/cmd/resource"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	return resource.NewCommand(&resource.CommandConfig{
		Use:              "models",
		Short:            "Manage model import/export",
		Long:             "Import and export model configurations from the project directory",
		ExportPath:       "/models/export",
		ImportPath:       "/models/import",
		SupportsStrategy: true,
	})
}
