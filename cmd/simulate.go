package cmd

import (
	"github.com/aporia-ai/kubesurvival/v2/pkg/logger"
	"github.com/aporia-ai/kubesurvival/v2/pkg/simulate"
	"github.com/spf13/cobra"
)

var simulateCmd = &cobra.Command{
	Use:   "simulate [config file]",
	Short: "Run simulations",
	Long:  `Run simulations based on the provided configuration file.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		configPath := args[0]

		cfg, err := simulate.ParseConfig(configPath)
		if err != nil {
			logger.Fatalf("[!] Could not read config file: %s", err)
		}
		if cfg.Nodes.MaxNodes == nil {
			podCount := int64(len(cfg.Pods))
			cfg.Nodes.MaxNodes = &podCount
		}

		pods, err := simulate.GeneratePods(cfg.Pods)
		if err != nil {
			logger.Fatalf("[!] Pod generation error: %s", err)
		}

		ns := simulate.ConfigureNodeSource(cfg)
		nodeTypes, err := ns.GetNodes()
		if err != nil {
			logger.Fatalf("Could not get node types: %s", err)
		}

		filteredNodeTypes := simulate.FilterNodeTypes(nodeTypes, pods)
		if len(filteredNodeTypes) == 0 {
			logger.Fatalf("[!] No nodes are available for simulation.")
		}
		logger.Infof("Running with %v Nodes", len(filteredNodeTypes))

		results, simRuns := simulate.RunSimulations(filteredNodeTypes, pods, cfg)
		simulate.DisplayResults(results, simRuns)
	},
}

func init() {
	rootCmd.AddCommand(simulateCmd)
}
