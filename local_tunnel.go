package main

import (
    "fmt"
    "log"
    "os/exec"
    "k8s.io/api/core/v1"
    "sync"
    "strings"
    "k8s.io/client-go/rest"
)

func createPortForwardTunnel(
    kubeConfigFile string,
    bastionPod *v1.Pod,
    localPort int32,
    remotePort int32,
    waitGroup *sync.WaitGroup,
    errorChannel *chan error,
    verbose bool,
) {
    log.Printf("Opening local kubectl port-forward tunnel localhost:%d => %s:%d...", localPort, bastionPod.Name, remotePort)
    command := exec.Command(
        "kubectl",
        fmt.Sprintf("--kubeconfig=%s", kubeConfigFile),
        "port-forward",
        bastionPod.Name,
        fmt.Sprintf("%d:%d", localPort, remotePort),
    )

    if verbose {
        log.Printf("About to exec: {%s}", strings.Join(command.Args, " "))
    }

    err := command.Run()
    if err != nil { *errorChannel<-err }

    waitGroup.Done()
}

func setupChiselClient(
    bastionPodName string,
    kubeConfig *rest.Config,
    remoteHost string,
    remotePort int32,
    localPort int32,
    tunnelPort int32,
    waitGroup *sync.WaitGroup,
    verbose bool,
) {
    log.Printf(
        "Started chisel client to forward traffic over kubectl tunnel to server localhost:%d => localhost:%d...",
        localPort,
        tunnelPort,
    )

    if verbose {
        connectionMap := strings.Join(
            []string{
                fmt.Sprintf("localhost:%d (chisel client)", localPort),
                fmt.Sprintf("localhost:%d (kubectl port-forward)", tunnelPort),
                fmt.Sprintf("%s:443 (kubernetes master)", kubeConfig.Host),
                fmt.Sprintf("%s:22 (chisel server pod)", bastionPodName),
                fmt.Sprintf("%s:%d", remoteHost, remotePort),
            },
            " => ",
        )

        log.Printf("Full Tunnel is:\n  %s", connectionMap)
    }

    command := exec.Command(
        "chisel",
        "client",
        fmt.Sprintf("localhost:%d", tunnelPort),
        fmt.Sprintf("localhost:%d:%s:%d", localPort, remoteHost, remotePort),
    )

    if verbose {
        log.Printf("About to exec: {%s}", strings.Join(command.Args, " "))
    }

    command.Run()
    waitGroup.Done()
}
