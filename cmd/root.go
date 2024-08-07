package cmd

import (
	"fmt"
	"os"

	"github.com/aporia-ai/kubesurvival/v2/pkg/logger"
	"github.com/spf13/cobra"
)

var logLevel string

var rootCmd = &cobra.Command{
	Use:   "kubesurvival",
	Short: "Your Project CLI",
	Long:  "A CLI tool for managing your project",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if err := logger.SetLogLevel(logLevel); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize()
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "error", "set the log level (debug, info, warn, error)")
}
