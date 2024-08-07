package cmd

import (
	"regexp"

	"github.com/aporia-ai/kubesurvival/v2/pkg/logger"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func totalPodCPU(pods []*corev1.Pod) string {
	totalCPU := resource.Quantity{}
	for _, pod := range pods {
		for _, cnt := range pod.Spec.Containers {
			totalCPU.Add(*cnt.Resources.Requests.Cpu())
		}
	}
	return totalCPU.String()
}
func totalPodMem(pods []*corev1.Pod) string {
	totalMem := resource.Quantity{}
	for _, pod := range pods {
		for _, cnt := range pod.Spec.Containers {
			logger.Logger().WithFields(logrus.Fields{
				"namespace": pod.Namespace,
				"pod":       pod.Name,
				"container": cnt.Name,
			}).Debugf("Adding %s ",
				cnt.Resources.Requests.Memory().String(),
				// cnt.Name, pod.Namespace, pod.Name,
			)
			totalMem.Add(*cnt.Resources.Requests.Memory())
		}
	}
	return totalMem.String()
}
func totalPodGPU(pods []*corev1.Pod) string {
	totalGPU := resource.Quantity{}
	for _, pod := range pods {
		for _, cnt := range pod.Spec.Containers {
			totalGPU.Add(cnt.Resources.Requests["nvidia.com/gpu"])
		}
	}
	return totalGPU.String()
}

func intPTR(i int) *int64 {
	a := int64(i)
	return &a
}

func matchesAny(value string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		matched, err := regexp.MatchString(pattern, value)
		if err != nil {
			logger.Errorf("Error matching regex pattern: %v", err)
			continue
		}
		if matched {
			return true
		}
	}
	return false
}
