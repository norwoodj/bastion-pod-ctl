package main

import (
    "os/signal"
    "syscall"
    "k8s.io/api/core/v1"
    "os"
    "log"
    "k8s.io/client-go/kubernetes"
    "sync"
    "k8s.io/client-go/rest"
    "os/exec"
    "strings"
    "time"
    "github.com/phayes/freeport"
    "fmt"
    "path"
)

var defaultSshArgs = []string{
    "-o", "StrictHostKeyChecking=no",
    "-o", "UserKnownHostsFile=/dev/null",
}

var handlerForSubcommand = map[string]func(options *bastionPodOptions){
    "start": startSubcommand,
    "forward": forwardSubcommand,
    "ssh": sshSubcommand,
}

func cleanup(kubeClient *kubernetes.Clientset, bastionPod *v1.Pod) {
    err := deleteBastionPod(kubeClient, bastionPod)

    for _, command := range backgroundCommands {
        if command.Process != nil {
            log.Printf("Killing background process %s, pid %d", path.Base(command.Path), command.Process.Pid)
            command.Process.Kill()
        }
    }

    if err != nil { panic(err) }
}

func setupExitHandlers(kubeClient *kubernetes.Clientset, bastionPod *v1.Pod) {
    gracefulStop := make(chan os.Signal)
    signal.Notify(gracefulStop, syscall.SIGTERM)
    signal.Notify(gracefulStop, syscall.SIGINT)

    go func() {
        sig := <-gracefulStop
        log.Printf("Caught Signal: %+v, exiting gracefully...", sig)
        cleanup(kubeClient, bastionPod)
        os.Exit(0)
    }()
}

func startBackgroundForwardingProcesses(
    options *bastionPodOptions,
    kubeClient *kubernetes.Clientset,
    kubeConfig *rest.Config,
    bastionPod *v1.Pod,
) (*chan error, int32) {
    var waitGroup sync.WaitGroup
    errorChannel := make(chan error, 1)

    if ! pollPodStatus(kubeClient, bastionPod) {
        os.Exit(1)
    }

    ephemeralPort0, _ := freeport.GetFreePort()
    kubectlTunnelPort := int32(ephemeralPort0)

    chiselClientPort := options.localPort
    if chiselClientPort < 0 {
        ephemeralPort1, _ := freeport.GetFreePort()
        chiselClientPort = int32(ephemeralPort1)
    }

    waitGroup.Add(1)
    go createPortForwardTunnel(
        options.kubeConfigFile,
        bastionPod,
        kubectlTunnelPort,
        defaultChiselServerPodPort,
        &waitGroup,
        &errorChannel,
        options.verbose,
    )

    waitGroup.Add(1)
    go setupChiselClient(
        bastionPod.Name,
        kubeConfig,
        options.remoteHost,
        options.remotePort,
        chiselClientPort,
        kubectlTunnelPort,
        &waitGroup,
        options.verbose,
    )

    go func() {
        waitGroup.Wait()
        close(errorChannel)
    }()

    return &errorChannel, chiselClientPort
}

func handleChildProcessErrors(
    errorChannel *chan error,
    kubeClient *kubernetes.Clientset,
    bastionPod *v1.Pod,
    verbose bool,
) {
    select {
        case err := <-*errorChannel:
            if err != nil {
                log.Printf("An Error occurred when starting background port forwarding processes, cleaning up...")
                cleanup(kubeClient, bastionPod)

                if verbose {
                    log.Printf("The error that occurred was: %s", err.Error())
                }
            }

            os.Exit(1)
    }
}

func forwardSubcommand(options *bastionPodOptions) {
    kubeClient, kubeConfig := getKubeClient(options.kubeConfigFile)
    bastionPod := createBastionPod(kubeClient)

    setupExitHandlers(kubeClient, bastionPod)
    errorChannel, chiselClientPort := startBackgroundForwardingProcesses(
        options,
        kubeClient,
        kubeConfig,
        bastionPod,
    )

    log.Printf("Sleeping 2 seconds for proxies to come up...")
    time.Sleep(2 * time.Second)

    log.Printf(
        "Running chisel tunnel localhost:%d => %s:%d... Press <CTRL-C> to exit",
        chiselClientPort,
        options.remoteHost,
        options.remotePort,
    )

    handleChildProcessErrors(errorChannel, kubeClient, bastionPod, options.verbose)
}

func sshSubcommand(options *bastionPodOptions) {
    kubeClient, kubeConfig := getKubeClient(options.kubeConfigFile)
    bastionPod := createBastionPod(kubeClient)

    setupExitHandlers(kubeClient, bastionPod)
    options.remotePort = 22
    errorChannel, chiselClientPort := startBackgroundForwardingProcesses(
        options,
        kubeClient,
        kubeConfig,
        bastionPod,
    )

    go handleChildProcessErrors(errorChannel, kubeClient, bastionPod, options.verbose)
    sshArgs := defaultSshArgs[:]
    sshArgs = append(sshArgs, "-p", fmt.Sprintf("%d", chiselClientPort))

    if options.verbose {
        sshArgs = append(sshArgs, "-v")
    }

    sshArgs = append(sshArgs, "localhost")
    command := exec.Command( "ssh", sshArgs...)

    log.Println("Sleeping 5 seconds to allow proxies to come up...")
    time.Sleep(5 * time.Second)

    log.Printf("Starting SSH session through localhost:%d", chiselClientPort)
    if options.verbose {
        log.Printf("About to exec: %s", strings.Join(command.Args, " "))
    }

    command.Stdout = os.Stdout
    command.Stderr = os.Stderr
    command.Stdin = os.Stdin
    err := command.Run()

    if err != nil {
        log.Printf("ERROR %s", err.Error())
    }

    log.Println("SSH connection terminated, deleting Bastion Pod...")
    cleanup(kubeClient, bastionPod)
}

func startSubcommand(options *bastionPodOptions) {
    kubeClient, _ := getKubeClient(options.kubeConfigFile)
    createBastionPod(kubeClient)
}

func main() {
    options, subcommand := parseCommandLine()
    handlerForSubcommand[subcommand](&options)
}
