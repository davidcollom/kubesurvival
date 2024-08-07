package simulate

import (
	"math"

	"github.com/aporia-ai/kubesurvival/v2/pkg/logger"
	"github.com/aporia-ai/kubesurvival/v2/pkg/nodesource"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func getInitialNodeCount(config *Config) int64 {
	if config.Nodes.MinNodes == nil {
		return 2
	}
	return *config.Nodes.MinNodes
}

func createNodeList(nodeType nodesource.Node, nodeCount int64) []nodesource.Node {
	nodes := make([]nodesource.Node, nodeCount)
	for i := 0; i < int(nodeCount); i++ {
		nodes[i] = nodeType
	}
	return nodes
}

func adjustNodeCount(nodeCount int64, config *Config) int64 {
	if config.Mode == "fast" {
		return nodeCount + int64(math.Max(float64(nodeCount)/15, 1))
	}
	return nodeCount + 1
}

func nodeResources(nodeType nodesource.Node) (resource.Quantity, resource.Quantity, resource.Quantity) {
	nodeConfig := nodeType.GetNodeConfig("node")
	nodeCpu := resource.MustParse(nodeConfig.Status.Allocatable["cpu"])
	nodeGpu := resource.MustParse(nodeConfig.Status.Allocatable["nvidia.com/gpu"])
	nodeMemory := resource.MustParse(nodeConfig.Status.Allocatable["memory"])
	return nodeCpu, nodeMemory, nodeGpu
}

func nodeHasEnoughResources(nodeType nodesource.Node, pods []*v1.Pod) bool {
	for _, pod := range pods {
		nodeCPU, nodeMemory, nodeGPU := nodeResources(nodeType)

		podCPU := resource.Quantity{}
		podMemory := resource.Quantity{}
		podGPU := resource.Quantity{}
		for _, cnt := range pod.Spec.Containers {
			podCPU.Add(*cnt.Resources.Requests.Cpu())
			podMemory.Add(*cnt.Resources.Requests.Memory())
			podGPU.Add(cnt.Resources.Requests["nvidia.com/gpu"])
		}

		if podCPU.Cmp(nodeCPU) > 0 {
			logger.Warnf("Ignoring node type %s with %s CPU because there's a pod with more CPU: %s\n",
				nodeType.GetInstanceType(), nodeCPU.String(), podCPU.String())
			return false
		}

		// podMemory := pod.Spec.Containers[0].Resources.Requests.Memory()
		if podMemory.Cmp(nodeMemory) > 0 {
			logger.Warnf("Ignoring node type %s with %s Memory because there's a pod with more memory: %s\n",
				nodeType.GetInstanceType(), nodeMemory.String(), podMemory.String())
			return false
		}

		// podGpu := pod.Spec.Containers[0].Resources.Requests["nvidia.com/gpu"]
		if nodeGPU.Cmp(nodeGPU) > 0 {
			logger.Warnf("Ignoring node type %s with %s GPU because there's a pod with more GPU: %s\n",
				nodeType.GetInstanceType(), nodeGPU.String(), podGPU.String())
			return false
		}
	}
	return true
}

func multiplyQuantity(q resource.Quantity, multiplier int64) resource.Quantity {
	// Convert the Quantity to MilliValue for precise arithmetic
	milliValue := q.MilliValue()

	// Perform the multiplication
	newMilliValue := milliValue * multiplier

	// Convert back to resource.Quantity
	return *resource.NewMilliQuantity(newMilliValue, q.Format)
}
