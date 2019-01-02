package kube

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func getKubeClientConfig(kubecontext string, kubeconfig string) (*rest.Config, error) {
    rules := clientcmd.NewDefaultClientConfigLoadingRules()
    rules.DefaultClientConfig = &clientcmd.DefaultClientConfig
    overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}

    if kubecontext != "" {
        overrides.CurrentContext = kubecontext
    }

    if kubeconfig != "" {
        rules.ExplicitPath = kubeconfig
    }

    config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()

	if err != nil {
		return nil, fmt.Errorf("could not get Kubernetes config for context %q: %s", kubecontext, err)
	}

	return config, nil
}

func GetKubernetesClient(kubecontext string, kubeconfig string) (*rest.Config, kubernetes.Interface, error) {
	config, err := getKubeClientConfig(kubecontext, kubeconfig)

	if err != nil {
		return nil, nil, err
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		return nil, nil, fmt.Errorf("could not get Kubernetes client: %s", err)
	}

	return config, client, nil
}
