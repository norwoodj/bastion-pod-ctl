package main

import (
    "fmt"
    "log"
    "os/exec"
    "sync"
    "strings"

    "k8s.io/api/core/v1"
    "k8s.io/client-go/rest"
)

var portForwardBackgroundCommand *exec.Cmd

func createPortForwardTunnel(
    kubeConfig *rest.Config,
    kubeConfigFile string,
    bastionPod *v1.Pod,
    localPort int32,
    remoteHost string,
    remotePort int32,
    waitGroup *sync.WaitGroup,
    errorChannel *chan error,
    verbose bool,
) {
    log.Printf("Opening local kubectl port-forward tunnel localhost:%d => %s:%d...", localPort, bastionPod.Name, defaultProxyServerPodPort)
    command := exec.Command(
        "kubectl",
        fmt.Sprintf("--namespace=%s", defaultBastionNamespace),
        fmt.Sprintf("--kubeconfig=%s", kubeConfigFile),
        "port-forward",
        bastionPod.Name,
        fmt.Sprintf("%d:%d", localPort, defaultProxyServerPodPort),
    )

    if verbose {
        log.Printf("About to exec: %s", strings.Join(command.Args, " "))
        connectionMap := strings.Join(
            []string{
                fmt.Sprintf("localhost:%d (kubectl port-forward)", localPort),
                fmt.Sprintf("%s:443 (kubernetes master)", kubeConfig.Host),
                fmt.Sprintf("%s:%d (bastion proxy pod)", bastionPod.Name, defaultProxyServerPodPort),
                fmt.Sprintf("%s:%d", remoteHost, remotePort),
            },
            " => ",
        )

        log.Printf("Full Tunnel is:\n  %s", connectionMap)
    }

    portForwardBackgroundCommand = command
    err := command.Run()
    if err != nil { *errorChannel<-err }
    waitGroup.Done()
}
