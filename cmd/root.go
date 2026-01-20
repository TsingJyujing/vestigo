package cmd

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var logger = logrus.New()

func init() {
	rootCmd.AddCommand(serverCommand)
	rootCmd.AddCommand(NewMcpCommand())
}

var rootCmd = &cobra.Command{
	Use:   "vestigo",
	Short: "Vestigo is a personal search engine",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
