package nodesource

import (
	"fmt"

	gcpinstancesinfo "github.com/davidcollom/gcp-instance-info"
	"github.com/pfnet-research/k8s-cluster-simulator/pkg/config"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GCPNode struct {
	InstanceType  string   `json:"instanceType"`
	OnDemandPrice float64  `json:"onDemandPriceUSD"`
	VCPU          int      `json:"vcpu"`
	Memory        float32  `json:"memory"`
	GPU           int      `json:"gpu"`
	MaxPods       int      `json:"maxPods"`
	Arch          []string `json:"arch"`
	// TODO: Add VolumeSize ondemand price.
}

type GCPNodeSource struct {
	Region              string
	InstanceTypes       []string
	VolumeSizePerNodeGB int64 // TODO
}

func (s *GCPNodeSource) Name() string {
	return "GCP"
}

func (s *GCPNodeSource) GetNodes() ([]Node, error) {
	var nodes []Node
	instances, err := gcpinstancesinfo.Data()
	if err != nil {
		return nil, errors.Wrap(err, "could not get ec2 instances info")
	}
	maxPodsPerInstance := 110 // TODO: Find a way of calculating Max num pods

	for _, instance := range instances.Compute.Instances {
		if v, ok := instance.Pricing[s.Region]; !ok || v.Hour == 0 {
			// We ignore instance if its not available in that region
			fmt.Printf("Instance %s is not available in %s\n", instance.InstanceType, s.Region)
			continue
		}

		nodes = append(nodes, &GCPNode{
			InstanceType:  instance.InstanceType,
			VCPU:          int(instance.VCPU),
			Memory:        float32(instance.Memory),
			GPU:           instance.GPU,
			MaxPods:       maxPodsPerInstance,
			OnDemandPrice: instance.Pricing[s.Region].Hour,
		})
	}

	return nodes, nil
}

func (s *GCPNodeSource) SetRegion(region string) {
	s.Region = region
}

func (s *GCPNodeSource) SetNodeTypes(nodeTypes []string) {
	s.InstanceTypes = nodeTypes
}

func (n *GCPNode) GetHourlyPrice() float64 {
	// TODO: Add storage price
	return n.OnDemandPrice
}

func (n *GCPNode) GetInstanceType() string {
	return n.InstanceType
}

func (n *GCPNode) GetNodeConfig(nodeName string) *config.NodeConfig {
	return &config.NodeConfig{
		Metadata: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				"beta.kubernetes.io/os": "simulated",
			},
		},
		Spec: v1.NodeSpec{
			Unschedulable: false,
		},
		Status: config.NodeStatus{
			Allocatable: map[v1.ResourceName]string{
				// We always assume free 10% vCPU and memory
				"cpu":            fmt.Sprintf("%dm", int(float32(n.VCPU)*1000*0.9)),
				"memory":         fmt.Sprintf("%dM", int(float64(n.Memory)*1024*0.9)),
				"nvidia.com/gpu": fmt.Sprintf("%d", n.GPU),
				"pods":           fmt.Sprintf("%d", n.MaxPods),
			},
		},
	}
}
