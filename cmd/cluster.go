package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aporia-ai/kubesurvival/v2/pkg/logger"
	"github.com/aporia-ai/kubesurvival/v2/pkg/simulate"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	excludeNamespaces []string
	excludePods       []string
	excludeContainers []string
	nodeTypes         []string
	provider          string
	providerRegion    string
	simMode           string
	configFile        string
	kubeconfig        string
	minNodes          int64
	maxNodes          int64
	spotInstance      bool
)

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Run simulations from an existing cluster",
	Long:  `Run simulations but using pod's from an existing cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		listPods()
	},
}

func init() {
	rootCmd.AddCommand(clusterCmd)

	clusterCmd.Flags().StringVar(&provider, "provider", "", "Specify the cloud provider (aws/gcp), auto-detect if not set")
	clusterCmd.Flags().StringVar(&providerRegion, "region", "", "Specify the cloud provider region (aws/gcp), auto-detect if not set")
	clusterCmd.Flags().BoolVar(&spotInstance, "spot", false, "Specify if you want to use Spot instances")
	clusterCmd.Flags().StringVar(&simMode, "mode", "fast", "Simulation mode, fast/slow - How NodeCounts are increased 15% or increments")
	clusterCmd.Flags().StringArrayVarP(&excludeNamespaces, "exclude-namespaces", "n", []string{}, "Comma-separated regex patterns to exclude namespaces")
	// clusterCmd.Flags().StringArrayVarP(&excludeNamespaces, "exclude-namespaces", "n", []string{"kube-system", "gke-managed-cim", "gke-managed-system", "gmp-public", "gmp-system", "configconnector-operator-system", "cnrm-system"}, "Comma-separated regex patterns to exclude namespaces")
	clusterCmd.Flags().StringArrayVarP(&excludePods, "exclude-pods", "p", []string{}, "Comma-separated regex patterns to exclude pod names")
	clusterCmd.Flags().StringArrayVarP(&excludeContainers, "exclude-containers", "c", []string{}, "Comma-separated regex patterns to exclude container names")
	clusterCmd.Flags().StringArrayVarP(&nodeTypes, "node-type", "N", []string{}, "Node Type to be included within the simulation")
	clusterCmd.Flags().StringVar(&configFile, "config", "", "Path to the configuration file")
	clusterCmd.Flags().Int64VarP(&minNodes, "min-nodes", "", 2, "Maximum nodes to simulate on")
	clusterCmd.Flags().Int64VarP(&maxNodes, "max-nodes", "", -1, "Maximum nodes to simulate on")
	if home := homedir.HomeDir(); home != "" {
		clusterCmd.Flags().StringVar(&kubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		clusterCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	}
}

func listPods() {
	var cfg *simulate.Config
	var err error
	// Use the kubeconfig file if specified
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		logger.Error("Error creating Kubernetes client config: ", err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Errorf("Error creating Kubernetes client: %v", err)
	}

	if provider == "" {
		provider, err = detectProvider(clientset)
		if err != nil {
			logger.Error("Could not detect cloud provider: ", err)
			os.Exit(1)
		}
		logger.Infof("Detected Provider as %s", provider)
	}

	if providerRegion == "" {
		providerRegion, err = detectRegion(clientset)
		if err != nil || providerRegion == "" {
			logger.Error("No Region provided, Unable to detect region from cluster: ", err)
			os.Exit(1)
		}
		logger.Infof("Detected Region as %s", providerRegion)
	}

	if configFile != "" {
		cfg, err = simulate.ParseConfig(configFile)
		if err != nil {
			logger.Error("Could not read config file: ", err)
			os.Exit(1)
		}
		if provider == "" {
			provider = cfg.Provider
		}
	} else {
		cfg = simulate.NewConfig(provider, providerRegion)
		cfg.Mode = simMode
	}
	cfg.Nodes.MinNodes = &minNodes
	// Need to find a better way of this instantiation
	cfg.Nodes.AWS.InstanceTypes = nodeTypes
	cfg.Nodes.GCP.InstanceTypes = nodeTypes

	pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error listing pods: %v", err)
	}

	var podsToKeep []*corev1.Pod
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			logger.Debugf("Removing %s/%s as its phase is: %s",
				pod.Namespace,
				pod.Name,
				pod.Status.Phase,
			)
			continue
		}

		if matchesAny(pod.Namespace, excludeNamespaces) {
			continue
		}
		if matchesAny(pod.Name, excludePods) {
			continue
		}

		nPod := corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      pod.Name,
				Namespace: pod.Namespace,
				Labels:    pod.Labels,
			},
			Spec: v1.PodSpec{
				Affinity: &corev1.Affinity{},
				// NodeSelector: pod.Spec.NodeSelector,
			},
		}
		// if pod.Spec.Affinity != nil && pod.Spec.Affinity.NodeAffinity != nil {
		// 	logger.Debugf("Adding NodeAffinity Rules for %s/%s: %+v",
		// 		pod.Namespace, pod.Name, pod.Spec.Affinity.NodeAffinity)
		// 	nPod.Spec.Affinity.NodeAffinity = pod.Spec.Affinity.NodeAffinity

		// 	// pretty.Print(*pod.Spec.Affinity)
		// 	// os.Exit(1)
		// }
		for _, container := range pod.Spec.Containers {
			if matchesAny(container.Name, excludeContainers) {
				continue
			}
			nPod.Spec.Containers = append(nPod.Spec.Containers, corev1.Container{
				Name:  container.Name,
				Image: container.Image,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    *container.Resources.Requests.Cpu(),
						corev1.ResourceMemory: *container.Resources.Requests.Memory(),
						"nvidia.com/gpu":      container.Resources.Requests["nvidia.com/gpu"],
					},
				},
			})
		}
		for _, container := range pod.Spec.InitContainers {
			if matchesAny(container.Name, excludeContainers) {
				continue
			}
			nPod.Spec.InitContainers = append(nPod.Spec.InitContainers, corev1.Container{
				Name:  container.Name,
				Image: container.Image,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    *container.Resources.Requests.Cpu(),
						corev1.ResourceMemory: *container.Resources.Requests.Memory(),
						"nvidia.com/gpu":      container.Resources.Requests["nvidia.com/gpu"],
					},
				},
			})
		}
		if len(pod.Spec.Containers) == 0 {
			logger.Debugf("No Containers within Pod %s/%s", pod.Namespace, pod.Name)
			continue
		}
		podsToKeep = append(podsToKeep, &nPod)
	}

	if len(podsToKeep) == 0 {
		log.Fatalf("Found %v Pods to Simulate with, Aborting", len(podsToKeep))
	}

	// If no Max is set, then we equal the number of Pods
	// If we can't do 1 pod per node, then whats the point of continuing?
	if maxNodes == -1 {
		logger.Debugf("Setting Max Nodes to match Number of Pods %v",
			len(podsToKeep))
		cfg.Nodes.MaxNodes = intPTR(len(podsToKeep))
	}

	fmt.Printf("Found %v Pods to Simulate, with %s CPU, %s Mem and %s GPU's",
		len(podsToKeep), totalPodCPU(podsToKeep), totalPodMem(podsToKeep), totalPodGPU(podsToKeep))

	ns := simulate.ConfigureNodeSource(cfg)
	ns.UseSpot(spotInstance)
	nodeTypes, err := ns.GetNodes()
	if err != nil {
		logger.Errorf("Could not get node types: %s", err)
		os.Exit(1)
	}
	filteredNodeTypes := simulate.FilterNodeTypes(nodeTypes, podsToKeep)
	if len(filteredNodeTypes) == 0 {
		logger.Errorf("[!] No nodes are available for simulation.")
		os.Exit(1)
	}
	results, simRuns := simulate.RunSimulations(filteredNodeTypes, podsToKeep, cfg)
	simulate.DisplayResults(results, simRuns)
}

func detectProvider(clientset *kubernetes.Clientset) (string, error) {
	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("error listing nodes: %w", err)
	}

	for _, node := range nodes.Items {
		for label := range node.Labels {
			if strings.Contains(label, "cloud.google.com") {
				return "gcp", nil
			}
		}
	}

	// Default to AWS if no GCP labels are found
	return "aws", nil
}

func detectRegion(clientset *kubernetes.Clientset) (string, error) {
	nodes, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("error listing nodes: %w", err)
	}

	for _, node := range nodes.Items {
		for label, value := range node.Labels {
			if strings.Contains(label, "topology.kubernetes.io/region") {
				return value, nil
			}
		}
	}

	// Default to AWS if no GCP labels are found
	return "", nil
}
