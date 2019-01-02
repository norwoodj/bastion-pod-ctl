package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/norwoodj/bastion-pod-ctl/pkg/kube"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var defaultSshArgs = []string{
	"-o", "StrictHostKeyChecking=no",
	"-o", "UserKnownHostsFile=/dev/null",
}


func cleanup(kubeClient kubernetes.Interface, bastionPod *v1.Pod, tunnel *kube.Tunnel) error {
	if tunnel != nil {
		tunnel.Close()
	}

	if bastionPod != nil {
		return kube.DeleteBastionPod(kubeClient, bastionPod)
	}

	return nil
}

func ensureKubernetesClient() (*rest.Config, kubernetes.Interface) {
	kubeConfig, kubeClient, err := kube.GetKubernetesClient(viper.GetString("kube-context"), viper.GetString("kube-config"))

	if err != nil {
		log.Errorf("Error creating kubernetes client %s", err)
		os.Exit(1)
	}

	return kubeConfig, kubeClient
}

func getRemotePort(remotePortString string) int {
	remotePort, err := strconv.Atoi(remotePortString)

	if err != nil {
		log.Errorf("Error parsing provided port number %s", remotePortString)
		os.Exit(1)
	}

	return remotePort
}

func createBastionPod(kubeClient kubernetes.Interface, remoteHost string, remotePort int) *v1.Pod {
	namespace := viper.GetString("namespace")
	bastionPod, err := kube.CreateBastionPod(kubeClient, remoteHost, remotePort, namespace)

	if err != nil {
		log.Errorf("Error creating bastion pod: %s", err)
		cleanup(kubeClient, bastionPod, nil)
		os.Exit(1)
	}

	err = kube.PollPodStatus(kubeClient, bastionPod)
	if err != nil {
		log.Error(err)
		cleanup(kubeClient, bastionPod, nil)
		os.Exit(1)
	}

	return bastionPod
}

func startPortForward(
	kubeClient kubernetes.Interface,
	kubeConfig *rest.Config,
	bastionPod *v1.Pod,
	localPort int,
) *kube.Tunnel {
	portForward := kube.NewTunnel(
		kubeClient.CoreV1().RESTClient(),
		kubeConfig,
		bastionPod.Namespace,
		bastionPod.Name,
		kube.ProxyServerPodPort,
	)

	err := portForward.ForwardPort(localPort)

	if err != nil {
		log.Error(err)
		cleanup(kubeClient, bastionPod, portForward)
		os.Exit(1)
	}

	return portForward
}

func forwardSubcommand(_ *cobra.Command, args []string) {
	remoteHost := args[0]
	remotePort := getRemotePort(args[1])

	kubeConfig, kubeClient := ensureKubernetesClient()
	bastionPod := createBastionPod(kubeClient, remoteHost, remotePort)

	localPort := viper.GetInt("local-port")
	if localPort < 0 {
		localPort = remotePort
	}

	portForward := startPortForward(kubeClient, kubeConfig, bastionPod, kube.ProxyServerPodPort)
	done := setupExitHandlers(kubeClient, bastionPod, portForward)

    log.Infof(
        "Running proxy tunnel localhost:%d => %s:%d... Press <CTRL-C> to exit",
        portForward.Local,
        remoteHost,
        remotePort,
    )

	<-done
}

func sshSubcommand(_ *cobra.Command, args []string) {
	remoteHost := args[0]
	remotePort := viper.GetInt("remote-port")

    kubeConfig, kubeClient := ensureKubernetesClient()
    bastionPod := createBastionPod(kubeClient, remoteHost, remotePort)

	portForward := startPortForward(kubeClient, kubeConfig, bastionPod, kube.ProxyServerPodPort)
    setupExitHandlers(kubeClient, bastionPod, portForward)

    sshArgs := defaultSshArgs[:]
    sshArgs = append(sshArgs, "-p", fmt.Sprintf("%d", portForward.Local))

    if viper.GetBool("verbose") {
		sshArgs = append(sshArgs, "-v")
	}

    sshArgs = append(sshArgs, "localhost")
    command := exec.Command("ssh", sshArgs...)

    log.Infof("Starting SSH session through localhost:%d", portForward.Local)

    command.Stdout = os.Stdout
    command.Stderr = os.Stderr
    command.Stdin = os.Stdin
    err := command.Run()

    if err != nil {
        log.Error(err)
    }

    log.Info("SSH connection terminated, deleting Bastion Pod...")
    cleanup(kubeClient, bastionPod, portForward)
}

func startSubcommand(_ *cobra.Command, args []string) {
	remoteHost := args[0]
	remotePort := viper.GetInt("remote-port")

	_, kubeClient := ensureKubernetesClient()
	createBastionPod(kubeClient, remoteHost, remotePort)
}