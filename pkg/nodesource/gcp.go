package nodesource

import (
	"fmt"

	"github.com/aporia-ai/kubesurvival/v2/pkg/logger"
	gcpinstancesinfo "github.com/davidcollom/gcp-instance-info"
	"github.com/pfnet-research/k8s-cluster-simulator/pkg/config"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GCPNode struct {
	InstanceType string   `json:"instanceType"`
	HourlyPrice  float64  `json:"hourlyPriceUSD"`
	VCPU         int      `json:"vcpu"`
	Memory       float32  `json:"memory"`
	GPU          int      `json:"gpu"`
	MaxPods      int      `json:"maxPods"`
	Arch         []string `json:"arch"`
	// TODO: Add VolumeSize ondemand price.
}

type GCPNodeSource struct {
	Region              string
	SpotInstance        bool
	InstanceTypes       []string
	VolumeSizePerNodeGB int64 // TODO
}

func (s *GCPNodeSource) Name() string {
	return "GCP"
}

func (s *GCPNodeSource) GetNodes() ([]Node, error) {
	var nodes []Node
	gcpdata, err := gcpinstancesinfo.Data()
	if err != nil {
		return nil, errors.Wrap(err, "could not get ec2 instances info")
	}
	maxPodsPerInstance := 110 // TODO: Find a way of calculating Max num pods

	instances := gcpdata.Compute.Instances
	if len(s.InstanceTypes) > 0 {
		//Create a new instances
		instances = map[string]gcpinstancesinfo.Instance{}
		for _, instanceName := range s.InstanceTypes {
			if ins, found := gcpdata.Compute.Instances[instanceName]; found {
				instances[instanceName] = ins
			}
		}
	}

	for _, instance := range instances {
		if v, ok := instance.Pricing[s.Region]; !ok || v.Hour == 0 {
			// We ignore instance if its not available in that region
			logger.Debugf("Instance %s is not available in %s", instance.InstanceType, s.Region)
			continue
		}

		hourlyPriceUSD := instance.Pricing[s.Region].Hour
		if s.SpotInstance {
			hourlyPriceUSD = instance.Pricing[s.Region].HourSpot
			if hourlyPriceUSD == 0 {
				logger.Debugf("Instance %s is not available as a Spot Instance %s", instance.InstanceType, s.Region)
				continue
			}
		}

		nodes = append(nodes, &GCPNode{
			InstanceType: instance.InstanceType,
			VCPU:         int(instance.VCPU),
			Memory:       float32(instance.Memory),
			GPU:          instance.GPU,
			MaxPods:      maxPodsPerInstance,
			HourlyPrice:  hourlyPriceUSD,
		})
	}

	return nodes, nil
}

func (s *GCPNodeSource) SetRegion(region string) {
	s.Region = region
}

func (s *GCPNodeSource) UseSpot(spot bool) {
	s.SpotInstance = spot
}

func (s *GCPNodeSource) SetNodeTypes(nodeTypes []string) {
	s.InstanceTypes = nodeTypes
}

func (n *GCPNode) GetHourlyPrice() float64 {
	// TODO: Add storage price
	return n.HourlyPrice
}

func (n *GCPNode) GetInstanceType() string {
	return n.InstanceType
}

func (n *GCPNode) GetNodeConfig(nodeName string) *config.NodeConfig {
	return &config.NodeConfig{
		Metadata: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				"beta.kubernetes.io/os":  "linux",
				"kubernetes.io/os":       "linux",
				"kubernetes.io/arch":     "amd64",
				"kubernetes.io/hostname": nodeName,
			},
		},
		Spec: v1.NodeSpec{
			Unschedulable: false,
		},
		Status: config.NodeStatus{
			Allocatable: map[v1.ResourceName]string{
				// We always assume free 10% vCPU and memory
				"cpu":    fmt.Sprintf("%dm", int(float32(n.VCPU)*1000*0.9)),
				"memory": fmt.Sprintf("%dMi", int(float64(n.Memory)*1024*0.9)),
				// "cpu":            fmt.Sprintf("%dm", int(float32(n.VCPU)*1000)),
				// "memory":         fmt.Sprintf("%dMi", int(float64(n.Memory)*1024)),
				"nvidia.com/gpu": fmt.Sprintf("%d", n.GPU),
				"pods":           fmt.Sprintf("%d", n.MaxPods),
			},
		},
	}
}
