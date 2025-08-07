package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "compozy-release",
	Short: "A CLI tool for managing Compozy releases",
	Long:  `compozy-release handles the entire release process, from version calculation to publishing.`,
}

func Execute() error {
	return rootCmd.Execute()
}
