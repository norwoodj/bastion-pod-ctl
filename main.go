package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
    "os/signal"
    "path"
    "sync"
    "strings"
    "syscall"
    "time"

    "github.com/phayes/freeport"
    "k8s.io/api/core/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
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

	if portForwardBackgroundCommand.Process != nil {
		log.Printf("Killing background process %s, pid %d", path.Base(portForwardBackgroundCommand.Path), portForwardBackgroundCommand.Process.Pid)
		portForwardBackgroundCommand.Process.Kill()
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

func waitForProxyStartup() {
    log.Println("Sleeping 2 seconds to allow proxy to come up...")
    time.Sleep(2 * time.Second)
}

func startBackgroundForwardingProcess(
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

    localTunnelPort := options.localPort
    if localTunnelPort < 0 {
        ephemeralPort, _ := freeport.GetFreePort()
        localTunnelPort = int32(ephemeralPort)
    }

    waitGroup.Add(1)
    go createPortForwardTunnel(
    	kubeConfig,
        options.kubeConfigFile,
        bastionPod,
        localTunnelPort,
        options.remoteHost,
        options.remotePort,
        &waitGroup,
        &errorChannel,
        options.verbose,
    )

    go func() {
        waitGroup.Wait()
        close(errorChannel)
    }()

    return &errorChannel, localTunnelPort
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
                log.Printf("An Error occurred when starting background port forwarding process, cleaning up...")
                cleanup(kubeClient, bastionPod)

                if verbose {
                    log.Printf("The error that occurred was: %s", err.Error())
                }
            }

            os.Exit(1)
    }
}

func forwardSubcommand(options *bastionPodOptions) {
	if options.localPort < 0 {
	    options.localPort = options.remotePort
	}

    kubeClient, kubeConfig := getKubeClient(options.kubeConfigFile)
    bastionPod := createBastionPod(kubeClient, options.remoteHost, options.remotePort)

    setupExitHandlers(kubeClient, bastionPod)
    errorChannel, localTunnelPort := startBackgroundForwardingProcess(
        options,
        kubeClient,
        kubeConfig,
        bastionPod,
    )

    waitForProxyStartup()
    log.Printf(
        "Running proxy tunnel localhost:%d => %s:%d... Press <CTRL-C> to exit",
        localTunnelPort,
        options.remoteHost,
        options.remotePort,
    )

    handleChildProcessErrors(errorChannel, kubeClient, bastionPod, options.verbose)
}

func sshSubcommand(options *bastionPodOptions) {
    if options.remotePort < 0 {
        options.remotePort = 22
    }

    kubeClient, kubeConfig := getKubeClient(options.kubeConfigFile)
    bastionPod := createBastionPod(kubeClient, options.remoteHost, options.remotePort)

    setupExitHandlers(kubeClient, bastionPod)
    errorChannel, localTunnelPort := startBackgroundForwardingProcess(
        options,
        kubeClient,
        kubeConfig,
        bastionPod,
    )

    go handleChildProcessErrors(errorChannel, kubeClient, bastionPod, options.verbose)
    sshArgs := defaultSshArgs[:]
    sshArgs = append(sshArgs, "-p", fmt.Sprintf("%d", localTunnelPort))

    if options.verbose {
        sshArgs = append(sshArgs, "-v")
    }

    sshArgs = append(sshArgs, "localhost")
    command := exec.Command("ssh", sshArgs...)
    waitForProxyStartup()

    log.Printf("Starting SSH session through localhost:%d", localTunnelPort)
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
    createBastionPod(kubeClient, options.remoteHost, options.remotePort)
}

func main() {
    options, subcommand := parseCommandLine()
    handlerForSubcommand[subcommand](&options)
}
