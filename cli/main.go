package main

import (
	"os"

	"github.com/compozy/compozy/cli/commands"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func main() {
	// Initialize logging
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.SetOutput(os.Stdout)

	// Create root command
	rootCmd := &cobra.Command{
		Use:   "compozy",
		Short: "Compozy - AI-powered workflow automation platform",
		Long:  "A command-line interface for managing Compozy workflows and projects.",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Set log level based on verbose flag
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				logrus.SetLevel(logrus.TraceLevel)
			} else {
				logrus.SetLevel(logrus.InfoLevel)
			}
			logrus.Info("Starting Compozy CLI")
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringP("cwd", "", "", "Current working directory")
	rootCmd.PersistentFlags().StringP("config", "", "compozy.yaml", "Path to the config file")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging")

	// Add subcommands
	rootCmd.AddCommand(commands.InitCmd())
	rootCmd.AddCommand(commands.BuildCmd())
	rootCmd.AddCommand(commands.DevCmd())
	rootCmd.AddCommand(commands.DeployCmd())

	// Execute
	if err := rootCmd.Execute(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
}
