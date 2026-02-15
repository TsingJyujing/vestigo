package main

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tsingjyujing/vestigo/cmd"
	"github.com/tsingjyujing/vestigo/utils"
)

var logger = utils.Logger

//go:embed version.txt
var version string

var versionCommand = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Vestigo",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "vestigo",
		Short: "Vestigo is a personal search engine",
	}
	for _, subCommand := range []*cobra.Command{
		cmd.NewServerCommand(),
		cmd.NewMcpCommand(),
		versionCommand,
	} {
		verboseOutput := false
		subCommand.Flags().BoolVarP(&verboseOutput, "verbose", "v", false, "Enable verbose output")
		subCommand.PersistentPreRun = func(cmd *cobra.Command, args []string) {
			if verboseOutput {
				utils.SetVerbose()
				logger.Debug("Verbose output enabled")
			}
		}
		rootCmd.AddCommand(subCommand)
	}
	if err := rootCmd.Execute(); err != nil {
		logger.WithError(err).Fatal("Failed to execute subCommand")
	}
}
