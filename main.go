package main

import (
	_ "embed"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tsingjyujing/vestigo/cmd"
)

var logger = logrus.New()

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
	rootCmd.Flags().BoolP("verbose", "v", false, "verbose output")
	commands := []*cobra.Command{
		cmd.NewServerCommand(),
		cmd.NewMcpCommand(),
		versionCommand,
	}
	for _, command := range commands {
		rootCmd.AddCommand(command)
	}
	if err := rootCmd.Execute(); err != nil {
		logger.WithError(err).Fatal("Failed to execute command")
	}
}
