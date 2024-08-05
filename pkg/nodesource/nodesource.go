package nodesource

import (
	"github.com/pfnet-research/k8s-cluster-simulator/pkg/config"
)

type Node interface {
	GetInstanceType() string
	GetHourlyPrice() float64
	GetNodeConfig(nodeName string) *config.NodeConfig
}

type NodeSource interface {
	Name() string
	SetRegion(string)
	SetNodeTypes([]string)
	GetNodes() ([]Node, error)
}

func GetNodeSource(provider string) NodeSource {
	var obj NodeSource
	switch provider {
	case "aws":
		obj = &AWSNodeSource{}
	case "gcp":
		obj = &GCPNodeSource{}
	}
	return obj
}
