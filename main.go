package main

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"sort"
	"time"

	"github.com/aporia-ai/kubesurvival/v2/pkg/kubesimulator"
	"github.com/aporia-ai/kubesurvival/v2/pkg/nodesource"
	"github.com/aporia-ai/kubesurvival/v2/pkg/parser"
	"github.com/aporia-ai/kubesurvival/v2/pkg/podgen"
	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
	"github.com/schollz/progressbar/v3"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Config struct {
	Provider string `yaml:"provider"`
	Mode     string `yaml:"mode"`
	Nodes    struct {
		MinNodes *int            `yaml:"minNodes"`
		MaxNodes *int            `yaml:"maxNodes"`
		GCP      *ProviderConfig `yaml:"gcp,omitempty"`
		AWS      *ProviderConfig `yaml:"aws,omitempty"`
	} `yaml:"nodes"`
	Pods string `yaml:"pods"`
}
type ProviderConfig struct {
	Region        string   `yaml:"region"`
	InstanceTypes []string `yaml:"instanceTypes"`
}

func (c *Config) GetProviderConfig() ProviderConfig {
	switch c.Provider {
	case "aws":
		return *c.Nodes.AWS
	case "gcp":
		return *c.Nodes.GCP
	default:
		return *c.Nodes.AWS
	}
}

type Result struct {
	InstanceType       string
	NodeCount          int
	TotalPricePerMonth float64
}

func main() {
	// Read argument
	if len(os.Args) != 2 {
		fmt.Println("USAGE: ./kubesurvival <YAML_CONFIG_PATH>")
		os.Exit(1)
	}

	// Read config file
	configFile, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("[!] Could not read config file: %s\n", err)
		return
	}

	// Parse config file
	config := &Config{}
	err = yaml.Unmarshal(configFile, config)
	if err != nil {
		fmt.Printf("[!] Could not deserialize config file: %s\n", err)
		return
	}

	// Parse & generate pods
	exp, parseErrors := parser.Parse(config.Pods)
	if len(parseErrors) > 0 {
		for _, parseError := range parseErrors {
			fmt.Printf("[!] Parse error: %s\n", parseError.Error())
		}
		return
	}

	pods, podgenErrors := podgen.Podgen(exp)
	if len(podgenErrors) > 0 {
		for _, podgenError := range podgenErrors {
			fmt.Printf("[!] PodGen error: %s\n", podgenError.Error())
		}
		return
	}

	// Generate nodes
	// ns := &nodesource.AWSNodeSource{
	// 	Region:     config.Nodes.AWS.Region,
	// 	InstanceTypes: config.Nodes.AWS.InstanceTypes,
	// }
	ns := nodesource.GetNodeSource(config.Provider)
	ns.SetRegion(config.GetProviderConfig().Region)
	ns.SetNodeTypes(config.GetProviderConfig().InstanceTypes)
	fmt.Printf("Using Provider: %s\n", ns.Name())

	nodeTypes, err := ns.GetNodes()
	if err != nil {
		fmt.Printf("Could not get node types: %s\n", err)
		return
	}

	// Remove node types if there's a pod with more resources than it
	filteredNodeTypes := filterNodeTypes(nodeTypes, pods)
	if len(filteredNodeTypes) == 0 {
		fmt.Printf("[!] No nodes are available for simulation.\n")
		return
	}
	fmt.Printf("Simulating (%v pods) with %v NodesTypes\n", len(pods), len(filteredNodeTypes))
	// Create a new progress bar
	bar := progressbar.NewOptions(len(filteredNodeTypes)+1,
		progressbar.OptionSetDescription("Simulating..."),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(false),
		progressbar.OptionSetWidth(15),
		progressbar.OptionClearOnFinish(),
		// progressbar.OptionShowIts(),
		progressbar.OptionThrottle(2*time.Second),
	)
	bar.Add(1) // Just so that we get the graph to show

	// Main loop
	var results []Result
	var simRuns int64
	// var maxtotalPricePerMonth float64
	for _, nodeType := range filteredNodeTypes {
		// We never want a cluster with only 1 node
		var nodeCount int
		if config.Nodes.MinNodes == nil {
			nodeCount = 2
		} else {
			nodeCount = *config.Nodes.MinNodes
		}

		for {
			// Calculate total price per month
			// fmt.Println(nodeType.GetHourlyPrice(), nodeType.GetInstanceType(), nodeCount)
			totalPricePerMonth := float64(nodeCount) * nodeType.GetHourlyPrice() * 24 * 31
			// fmt.Println(nodeType.GetInstanceType(), maxtotalPricePerMonth, totalPricePerMonth)

			// Do we even need to simulate?
			// if totalPricePerMonth > maxtotalPricePerMonth {
			// 	break
			// }

			// Generate a list of nodes from this type
			nodes := []nodesource.Node{}
			for i := 0; i < nodeCount; i++ {
				nodes = append(nodes, nodeType)
			}

			// Simulate cluster
			simulator := &kubesimulator.KubernetesSimulator{}
			isSimulationSuccessful, err := simulator.Simulate(pods, nodes)
			simRuns++
			if err != nil {
				fmt.Printf("[!] Failed to simulate a Kubernetes cluster: %s\n", err)
				// return
			}

			if isSimulationSuccessful {
				results = append(results, Result{
					InstanceType:       nodeType.GetInstanceType(),
					NodeCount:          nodeCount,
					TotalPricePerMonth: totalPricePerMonth,
				})
				bar.Add(1)
				// fmt.Printf("Successfully Scheduled on %s, %v, %.2f\n", nodeType.GetInstanceType(), nodeCount, totalPricePerMonth)
				break
			}

			// Simple heuristic as an alternative to nodeCount++ to make convergence faster.

			if config.Mode == "fast" {
				nodeCount += int(math.Max(float64(nodeCount)/15, 1))
			} else {
				nodeCount++
			}

			if config.Nodes.MaxNodes != nil && nodeCount >= *config.Nodes.MaxNodes {
				bar.Add(1)
				fmt.Printf("Instance type %s reached limit of 1000 Nodes\n", nodeType.GetInstanceType())
				break
			}
		}
	}

	if len(results) != 0 {
		fmt.Printf("Completed %s Simulations!\n", humanize.Comma(simRuns))
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Instance Type", "Node Count", "Price/month (USD)"})

		sort.Slice(results, func(i, j int) bool {
			if results[i].TotalPricePerMonth == results[j].TotalPricePerMonth {
				return results[i].NodeCount < results[j].NodeCount
			}
			return results[i].TotalPricePerMonth < results[j].TotalPricePerMonth
		})

		for _, v := range results {
			table.Append([]string{v.InstanceType, fmt.Sprintf("%v", v.NodeCount), humanize.FormatFloat("#,###.###", v.TotalPricePerMonth)})
		}
		table.Render()
	} else {
		fmt.Printf("[!] Could not converge to a solution over %v Simulations.\n", simRuns)
	}
}

func filterNodeTypes(nodeTypes []nodesource.Node, pods []*v1.Pod) []nodesource.Node {
	var result []nodesource.Node
	for _, nodeType := range nodeTypes {
		// nodeType = nodeType.(*nodesource.AWSNode)
		nodeHasEnoughResources := true

		for _, pod := range pods {
			// Is Pod CPU > Node CPU?
			nodeConfig := nodeType.GetNodeConfig("node")
			nodeCpu := resource.MustParse(nodeConfig.Status.Allocatable["cpu"])
			podCpu := pod.Spec.Containers[0].Resources.Requests.Cpu()
			if podCpu.Cmp(nodeCpu) > 0 {
				fmt.Printf("WARNING: Ignoring node type %s with %s CPU because there's a pod with more CPU: %s\n",
					nodeType.GetInstanceType(), nodeCpu.String(), podCpu.String())
				nodeHasEnoughResources = false
				break
			}

			// Is Pod Memory > Node Memory?
			nodeMemory := resource.MustParse(nodeConfig.Status.Allocatable["memory"])
			podMemory := pod.Spec.Containers[0].Resources.Requests.Memory()
			if podMemory.Cmp(nodeMemory) > 0 {
				fmt.Printf("WARNING: Ignoring node type %s with %s Memory because there's a pod with more memory: %s\n",
					nodeType.GetInstanceType(), nodeMemory.String(), podMemory.String())
				nodeHasEnoughResources = false
				break
			}

			// Is Pod Memory > Node Memory?
			nodeGpu := resource.MustParse(nodeConfig.Status.Allocatable["nvidia.com/gpu"])
			podGpu := pod.Spec.Containers[0].Resources.Requests["nvidia.com/gpu"]
			if podGpu.Cmp(nodeGpu) > 0 {
				fmt.Printf("WARNING: Ignoring node type %s with %s GPU because there's a pod with more GPU: %s\n",
					nodeType.GetInstanceType(), nodeGpu.String(), podGpu.String())
				nodeHasEnoughResources = false
				break
			}
		}

		if nodeHasEnoughResources {
			result = append(result, nodeType)
		}
	}

	return result
}
