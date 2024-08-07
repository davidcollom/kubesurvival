package simulate

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/aporia-ai/kubesurvival/v2/pkg/kubesimulator"
	"github.com/aporia-ai/kubesurvival/v2/pkg/logger"
	"github.com/aporia-ai/kubesurvival/v2/pkg/nodesource"
	"github.com/aporia-ai/kubesurvival/v2/pkg/parser"
	"github.com/aporia-ai/kubesurvival/v2/pkg/podgen"
	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
	"github.com/schollz/progressbar/v3"
	v1 "k8s.io/api/core/v1"
)

func GeneratePods(podsConfigPath string) ([]*v1.Pod, error) {
	exp, parseErrors := parser.Parse(podsConfigPath)
	if len(parseErrors) > 0 {
		for _, parseError := range parseErrors {
			logger.Errorf("[!] Parse error: %s", parseError.Error())
		}
		return nil, fmt.Errorf("parse errors encountered")
	}

	pods, podgenErrors := podgen.Podgen(exp)
	if len(podgenErrors) > 0 {
		for _, podgenError := range podgenErrors {
			logger.Errorf("[!] PodGen error: %s", podgenError.Error())
		}
		return nil, fmt.Errorf("pod generation errors encountered")
	}

	return pods, nil
}

func ConfigureNodeSource(config *Config) nodesource.NodeSource {
	ns := nodesource.GetNodeSource(config.Provider)
	providerConfig := config.GetProviderConfig()
	ns.SetRegion(providerConfig.Region)
	ns.SetNodeTypes(providerConfig.InstanceTypes)
	logger.Infof("Using Provider: %s with %v Instance Types", ns.Name(), len(providerConfig.InstanceTypes))
	return ns
}

func RunSimulations(nodeTypes []nodesource.Node, pods []*v1.Pod, config *Config) ([]Result, int64) {
	bar := progressbar.NewOptions(len(nodeTypes),
		progressbar.OptionSetDescription("Simulating..."),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(false),
		progressbar.OptionSetWidth(15),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionThrottle(2*time.Second),
	)
	bar.RenderBlank()

	var results []Result
	var simRuns int64

	for _, nodeType := range nodeTypes {
		nodeCount := getInitialNodeCount(config)

		for {
			totalPricePerMonth := float64(nodeCount) * nodeType.GetHourlyPrice() * 24 * 31

			nCPUe, nMeme, nGPUe := nodeResources(nodeType)
			nCPU := multiplyQuantity(nCPUe, nodeCount)
			nMem := multiplyQuantity(nMeme, nodeCount)
			nGPU := multiplyQuantity(nGPUe, nodeCount)

			logger.Debugf("Simulating %v Nodes of %s - Totalling %s(%s) CPU, %s(%s) Mem and %s(%s) GPU",
				nodeCount, nodeType.GetInstanceType(),
				nCPU.String(),
				nCPUe.String(),

				nMem.String(),
				nMeme.String(),
				nGPU.String(),
				nGPUe.String(),
			)

			nodes := createNodeList(nodeType, nodeCount)
			simulator := &kubesimulator.KubernetesSimulator{}
			isSimulationSuccessful, err := simulator.Simulate(pods, nodes)
			simRuns++
			if err != nil {
				logger.Warnf("[!] Failed to simulate a Kubernetes cluster: %s", err)
				break
			}

			if isSimulationSuccessful {
				results = append(results, Result{
					InstanceType:       nodeType.GetInstanceType(),
					NodeCount:          nodeCount,
					TotalPricePerMonth: totalPricePerMonth,
				})
				err = bar.Add(1)
				if err != nil {
					logger.Error(err)
				}
				break
			}

			nodeCount = adjustNodeCount(nodeCount, config)
			if config.Nodes.MaxNodes != nil && nodeCount >= *config.Nodes.MaxNodes {
				err = bar.Add(1)
				if err != nil {
					logger.Error(err)
				}
				logger.Warnf("Instance type %s reached limit of %d Nodes", nodeType.GetInstanceType(), *config.Nodes.MaxNodes)
				break
			}
		}
	}

	return results, simRuns
}

func DisplayResults(results []Result, simRuns int64) {
	if len(results) == 0 {
		logger.Errorf("[!] Could not converge to a solution over %v simulations.", simRuns)
		return
	}
	fmt.Println() // So that the progress bar doens't break
	logger.Infof("Completed %s simulations!", humanize.Comma(simRuns))
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Instance Type", "Node Count", "Price/month (USD)"})

	sort.Slice(results, func(i, j int) bool {
		if results[i].TotalPricePerMonth == results[j].TotalPricePerMonth {
			return results[i].NodeCount < results[j].NodeCount
		}
		return results[i].TotalPricePerMonth < results[j].TotalPricePerMonth
	})

	for _, result := range results {
		table.Append([]string{result.InstanceType, fmt.Sprintf("%v", result.NodeCount), humanize.FormatFloat("#,###.###", result.TotalPricePerMonth)})
	}
	table.Render()
}

func FilterNodeTypes(nodeTypes []nodesource.Node, pods []*v1.Pod) []nodesource.Node {
	var result []nodesource.Node
	for _, nodeType := range nodeTypes {
		if nodeHasEnoughResources(nodeType, pods) {
			result = append(result, nodeType)
		}
	}
	// Try and sort instance types into smallest first?
	sort.Slice(result, func(a, b int) bool {
		aC, aM, aG := nodeResources(result[a])
		bC, bM, bG := nodeResources(result[b])

		// Add all the values to CPU
		aC.Add(aM)
		aC.Add(aG)

		// Add all the values to CPU
		bC.Add(bM)
		bC.Add(bG)

		// we first compare only based on the family
		switch aC.Cmp(bC) {
		case -1:
			return true
		case 1:
			return false
		}

		return false
	})
	return result
}
