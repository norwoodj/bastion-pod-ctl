package main

import (
    "fmt"
    "log"
    "os"
    "time"

    apiv1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/client-go/rest"
)

const defaultProxyServerPodPort = 8080
const defaultPodStatusPollInterval = 3 * time.Second
const defaultBastionNamespace = "bastion"


func getBastionPodName() string {
    return fmt.Sprintf("bastion-%s-%s", os.Getenv("USER"), time.Now().UTC().Format("20060102-150405"))
}

func getBastionNamespaceObject(namespaceName string) apiv1.Namespace {
    return apiv1.Namespace{
        ObjectMeta: metav1.ObjectMeta{
            Name: namespaceName,
        },
    }
}

func getBastionPodObject(podName string, remoteHost string, remotePort int32) apiv1.Pod {
    return apiv1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name: podName,
        },
        Spec: apiv1.PodSpec{
            Containers: []apiv1.Container{{
                Name:  "bastion-proxy",
                Image: "tecnativa/tcp-proxy",
                Env: []apiv1.EnvVar {
                    {Name: "LISTEN", Value: fmt.Sprintf(":%d", defaultProxyServerPodPort)},
                    {Name: "TALK", Value: fmt.Sprintf("%s:%d", remoteHost, remotePort)},
                },
                Ports: []apiv1.ContainerPort{
                    {Protocol: apiv1.ProtocolTCP, ContainerPort: defaultProxyServerPodPort},
                },
            }},
        },
    }
}

func getKubeClient(kubeConfigFile string) (*kubernetes.Clientset, *rest.Config) {
    // use the current context in kubeconfig
    kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigFile)
    if err != nil {
        fmt.Printf("Error configuring kubectl with provided config. Ensure that `current-context` is set in %s\n", kubeConfigFile)
        panic(err.Error())
    }

    kubeClient, err := kubernetes.NewForConfig(kubeConfig)
    if err != nil { panic(err.Error()) }

    return kubeClient, kubeConfig
}

func createBastionPod(kubeClient *kubernetes.Clientset, remoteHost string, remotePort int32) *apiv1.Pod {
    bastionPodName := getBastionPodName()
    log.Printf("Creating Bastion Pod %s to forward traffic :%d => %s:%d", bastionPodName, defaultProxyServerPodPort, remoteHost, remotePort)

    namespacesClient := kubeClient.CoreV1().Namespaces()
    namespaceObject := getBastionNamespaceObject(defaultBastionNamespace)
    namespacesClient.Create(&namespaceObject)

    podsClient := kubeClient.CoreV1().Pods(defaultBastionNamespace)
    podObject := getBastionPodObject(bastionPodName, remoteHost, remotePort)
    pod, err := podsClient.Create(&podObject)
    if err != nil { panic(err) }

    return pod
}

func pollPodStatus(kubeClient *kubernetes.Clientset, bastionPod *apiv1.Pod) bool {
    podsClient := kubeClient.CoreV1().Pods(defaultBastionNamespace)
    bastionPodUpdate, err := podsClient.Get(bastionPod.Name, metav1.GetOptions{})
    if err != nil { panic(err) }

    for bastionPodUpdate.Status.Phase == apiv1.PodPending {
        log.Printf("Bastion Pod %s is still in %s state, will poll again shortly...", bastionPod.Name, apiv1.PodPending)
        time.Sleep(defaultPodStatusPollInterval)
        bastionPodUpdate, err = podsClient.Get(bastionPod.Name, metav1.GetOptions{})
        if err != nil { panic(err) }
    }

    if bastionPodUpdate.Status.Phase == apiv1.PodRunning {
        log.Printf("Bastion Pod %s now in %s state, continuing", bastionPod.Name, apiv1.PodRunning)
        return true
    }

    log.Printf("Bastion Pod %s entered bad state %s, deleting it and exiting", bastionPod.Name, bastionPodUpdate.Status.Phase)
    deleteBastionPod(kubeClient, bastionPodUpdate)
    return false
}

func deleteBastionPod(kubeClient *kubernetes.Clientset, bastionPod *apiv1.Pod) error {
    log.Printf("Deleting Bastion Pod %s...", bastionPod.Name)
    podsClient := kubeClient.CoreV1().Pods(defaultBastionNamespace)
    return podsClient.Delete(bastionPod.Name, nil)
}
