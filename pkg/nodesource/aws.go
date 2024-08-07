package nodesource

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	ec2instancesinfo "github.com/cristim/ec2-instances-info"
	"github.com/pfnet-research/k8s-cluster-simulator/pkg/config"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:generate curl -o eni-max-pods.txt https://raw.githubusercontent.com/aws/amazon-vpc-cni-k8s/master/misc/eni-max-pods.txt
//go:embed eni-max-pods.txt
var maxPodsData []byte

type AWSNode struct {
	InstanceType  string   `json:"instanceType"`
	OnDemandPrice float64  `json:"onDemandPriceUSD"`
	VCPU          int      `json:"vcpu"`
	Memory        float32  `json:"memory"`
	GPU           int      `json:"gpu"`
	MaxPods       int      `json:"maxPods"`
	Arch          []string `json:"arch"`
	// TODO: Add VolumeSize ondemand price.
}

type AWSNodeSource struct {
	Region              string
	InstanceTypes       []string
	VolumeSizePerNodeGB int64 // TODO
}

func (s *AWSNodeSource) Name() string {
	return "AWS"
}

func (s *AWSNodeSource) UseSpot(bool) {}

func (s *AWSNodeSource) SetRegion(region string) {
	s.Region = region
}
func (s *AWSNodeSource) SetNodeTypes(nodeTypes []string) {
	s.InstanceTypes = nodeTypes
}

func (s *AWSNodeSource) GetNodes() ([]Node, error) {
	instances, err := ec2instancesinfo.Data()
	if err != nil {
		return nil, errors.Wrap(err, "could not get ec2 instances info")
	}

	maxPodsPerInstance, err := s.getMaxPodsPerInstance()
	if err != nil {
		return nil, errors.Wrap(err, "could not get max pods per instance")
	}

	var nodes []Node

	for _, instanceType := range s.InstanceTypes {
		// Find max pods for this instance
		maxPods, ok := maxPodsPerInstance[instanceType]
		if !ok {
			fmt.Printf("could not find max pods for instance: %s, assuming Default of 110", instanceType)
			maxPods = 110
		}

		// Find info for this instance
		found := false
		for _, instance := range *instances {
			if instanceType == instance.InstanceType {
				fmt.Printf("Adding %s to list\n", instance.InstanceType)
				nodes = append(nodes, &AWSNode{
					InstanceType:  instance.InstanceType,
					OnDemandPrice: instance.Pricing[s.Region].Linux.OnDemand,
					VCPU:          instance.VCPU,
					Memory:        instance.Memory,
					GPU:           instance.GPU,
					Arch:          instance.Arch,
					MaxPods:       maxPods,
				})

				found = true
				break
			}
		}

		if !found {
			return nil, errors.New(fmt.Sprintf("Could not find instance data for %s", instanceType))
		}
	}

	return nodes, nil
}

func (s *AWSNodeSource) getMaxPodsPerInstance() (map[string]int, error) {
	maxPodsPerInstance := map[string]int{}

	scanner := bufio.NewScanner(bytes.NewReader(maxPodsData))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

		splitted := strings.Split(line, " ")
		if len(splitted) != 2 {
			return nil, errors.Errorf("could not parse eni-max-pods.txt file, bad line: %s", line)
		}

		instanceType := splitted[0]
		maxPods, err := strconv.ParseInt(splitted[1], 10, 32)
		if err != nil {
			return nil, errors.Errorf("could not parse eni-max-pods.txt file, bad line: %s", line)
		}

		maxPodsPerInstance[instanceType] = int(maxPods)
	}
	return maxPodsPerInstance, nil
}

func (n *AWSNode) GetHourlyPrice() float64 {
	// TODO: Add storage price
	return n.OnDemandPrice
}

func (n *AWSNode) GetInstanceType() string {
	return n.InstanceType
}

func (n *AWSNode) GetNodeConfig(nodeName string) *config.NodeConfig {
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
