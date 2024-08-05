package kubesimulator

import (
	"context"
	"fmt"
	"time"

	"github.com/aporia-ai/kubesurvival/v2/pkg/nodesource"
	kubesim "github.com/pfnet-research/k8s-cluster-simulator/pkg"
	"github.com/pfnet-research/k8s-cluster-simulator/pkg/config"
	"github.com/pfnet-research/k8s-cluster-simulator/pkg/queue"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
)

type KubernetesSimulator struct {
}

func (s *KubernetesSimulator) Simulate(pods []*v1.Pod, nodes []nodesource.Node) (bool, error) {
	queue := queue.NewPriorityQueue()
	sched := buildScheduler()

	nodeConfigs := []config.NodeConfig{}
	for i, node := range nodes {
		nodeName := fmt.Sprintf("node-%d", i)
		nodeConfigs = append(nodeConfigs, *node.GetNodeConfig(nodeName))
	}

	// file, _ := os.Create(fmt.Sprintf("./%s", nodes[0].GetInstanceType()))
	// _ = file

	clusterConfig := &config.Config{
		LogLevel: "info",
		// LogLevel:      "debug",
		StartClock:    time.Now().Format(time.RFC3339),
		Tick:          10,
		MetricsTick:   60,
		MetricsLogger: []config.MetricsLoggerConfig{},
		// MetricsLogger: []config.MetricsLoggerConfig{{Dest: "stderr", Formatter: "humanReadable"}},
		// MetricsLogger: []config.MetricsLoggerConfig{{Dest: file.Name(), Formatter: "humanReadable"}},
		// MetricsLogger: []config.MetricsLoggerConfig{{Dest: fmt.Sprintf("./%s.log", nodes[0].GetInstanceType()), Formatter: "humanReadable"}},
		// MetricsLogger: []config.MetricsLoggerConfig{{Dest: fmt.Sprintf("./%s-%s.log", nodes[0].GetInstanceType(), time.Now()), Formatter: "humanReadable"}},
		Cluster: nodeConfigs,
	}

	kubesim, err := kubesim.NewKubeSim(clusterConfig, queue, sched)
	if err != nil {
		return false, errors.Wrap(err, "failed to create kubesim")
	}

	kubesim.AddSubmitter("Submitter", newSubmitter(pods))

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	err = kubesim.Run(ctx)
	if err != nil && errors.Cause(err) != context.DeadlineExceeded {
		return false, errors.Wrap(err, "failed to run kubesim")
	}

	return errors.Cause(err) != context.DeadlineExceeded && (queue.Metrics().PendingPodsNum == 0), nil
}
