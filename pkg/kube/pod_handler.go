package kube

import (
	"fmt"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/api/resource"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

const ProxyServerPodPort = 8080
const BastionPodSelector = "app.kubernetes.io/managed-by==bastion-pod-ctl"

func getBastionPodName() string {
	return fmt.Sprintf("bastion-%s-%s", os.Getenv("USER"), time.Now().UTC().Format("20060102-150405"))
}

func getBastionPodLabels(podName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/component":  "cluster",
		"app.kubernetes.io/managed-by": "bastion-pod-ctl",
		"app.kubernetes.io/name":       podName,
		"app.kubernetes.io/part-of":    "bastion-pod-ctl",
	}
}

func getBastionPodObject(podName string, remoteHost string, remotePort int) apiv1.Pod {
	cpuRequests := viper.GetString("cpu-request")
	memoryRequests := viper.GetString("memory-request")
	podResources := apiv1.ResourceRequirements{
		Requests: apiv1.ResourceList{
			apiv1.ResourceCPU:    resource.MustParse(cpuRequests),
			apiv1.ResourceMemory: resource.MustParse(memoryRequests),
		},
		Limits: apiv1.ResourceList{
			apiv1.ResourceCPU:    resource.MustParse(cpuRequests),
			apiv1.ResourceMemory: resource.MustParse(memoryRequests),
		},
	}

	return apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   podName,
			Labels: getBastionPodLabels(podName),
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{{
				Name:  "bastion-proxy",
				Image: "alpine/socat",
				Args: []string{
					fmt.Sprintf("tcp-l:%d,fork,reuseaddr", ProxyServerPodPort),
					fmt.Sprintf("tcp:%s:%d", remoteHost, remotePort),
				},
				Ports: []apiv1.ContainerPort{
					{Protocol: apiv1.ProtocolTCP, ContainerPort: ProxyServerPodPort},
				},
				Resources: podResources,
			}},
		},
	}
}

func CreateBastionPod(kubeClient kubernetes.Interface, remoteHost string, remotePort int, namespace string) (*apiv1.Pod, error) {
	bastionPodName := getBastionPodName()
	log.Printf("Creating Bastion Pod %s to forward traffic :%d => %s:%d", bastionPodName, ProxyServerPodPort, remoteHost, remotePort)

	podsClient := kubeClient.CoreV1().Pods(namespace)
	podObject := getBastionPodObject(bastionPodName, remoteHost, remotePort)
	pod, err := podsClient.Create(&podObject)

	if err != nil {
		return nil, err
	}

	return pod, nil
}

func PollPodStatus(kubeClient kubernetes.Interface, bastionPod *apiv1.Pod) error {
	bastionPodWatch, err := kubeClient.CoreV1().
		Pods(bastionPod.Namespace).
		Watch(metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name==%s", bastionPod.Name)})

	if err != nil {
		log.Error("Error watching bastion pod status")
		return err
	}

	for event := range bastionPodWatch.ResultChan() {
		pod, ok := event.Object.(*apiv1.Pod)
		if !ok {
			return fmt.Errorf("unexpected type returned in pod watch request")
		}

		switch event.Type {
		case watch.Modified:
			if pod.Status.Phase == apiv1.PodRunning {
				log.Infof("Bastion Pod %s now in %s state, continuing", bastionPod.Name, apiv1.PodRunning)
				return nil
			} else if pod.Status.Phase == apiv1.PodPending {
				log.Infof("Bastion Pod %s is still in %s state, will poll again shortly...", bastionPod.Name, apiv1.PodPending)
			} else {
				log.Infof("Bastion Pod %s entered bad state %s, deleting it and exiting", bastionPod.Name, bastionPod.Status.Phase)
				DeleteBastionPod(kubeClient, bastionPod)
				return fmt.Errorf(
					"bastion Pod %s entered bad state %s",
					bastionPod.Name,
					pod.Status.Phase,
				)
			}
		}
	}

	return nil
}

func DeleteBastionPod(kubeClient kubernetes.Interface, bastionPod *apiv1.Pod) error {
	log.Printf("Deleting Bastion Pod %s...", bastionPod.Name)
	podsClient := kubeClient.CoreV1().Pods(bastionPod.Namespace)
	return podsClient.Delete(bastionPod.Name, nil)
}
