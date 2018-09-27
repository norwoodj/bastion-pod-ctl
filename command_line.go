package main

import (
    "fmt"
    flag "github.com/spf13/pflag"
    "log"
    "os"
    "path/filepath"

    "k8s.io/client-go/util/homedir"
)

type bastionPodOptions struct {
    help           bool
    kubeConfigFile string
    localPort      int32
    remoteHost     string
    remotePort     int32
    verbose        bool
}

var defaultKubeConfigPath = filepath.Join(homedir.HomeDir(), ".kube", "config")

const helpOptionHelp = "Print this help menu, then exist"
const forwardRemotePortHelp = "The port of the remote host we're tunneling tcp traffic to (required)"
const forwardLocalPortHelp = "The local port of the tunnel proxying traffic to the private remote host (default = ${remote-port})"
const kubeConfigOptionHelp = "Supply path to a kubeconfig file to use in authenticating to the kubernetes cluster API (default = ~/.kube/config)"
const remoteOptionHelp = "The remote host to which this will tunnel a tcp connection through a bastion pod (required)"
const sshRemotePortHelp = "The ssh port of the remote host we're tunneling to (default = 22)"
const verboseOptionHelp = "Print verbose output"

const forwardSubcommandHelp = "Open a TCP tunnel through the created bastion pod to the specified private remote host"
const sshSubcommandHelp = "SSH through the created bastion pod to the specified private remote host"
const startSubcommandHelp = "For debugging - starts up a bastion pod in the specified cluster and then exits, leaving it running"

const remoteHostRequiredMsg = "-r or --remote option is required!"
const remotePortRequiredMsg = "-p or --remote-port option is required!"

var helpForSubcommand = map[string]func(){
    "forward": printForwardSubcommandHelp,
    "ssh": printSshSubcommandHelp,
    "start": printStartSubcommandHelp,
}

var checkOptionsForSubcommand = map[string]func(*bastionPodOptions){
    "forward": checkForwardOptions,
    "ssh": checkSshOptions,
    "start": func(*bastionPodOptions) {},
}

func printBaseCommandHelp() {
    fmt.Println("Usage:")
    fmt.Println("  bastion-pod-ctl [options] [subcommand]")
    fmt.Println()
    fmt.Println("bastion-pod-ctl creates a pod running a proxy forwarding TCP traffic to a specified address. Subcommands")
    fmt.Println("can be used to ssh through this tunnel or to leave the tunnel open for other applications to proxy")
    fmt.Println("traffic to the private address")
    fmt.Println()
    fmt.Println("Options:")
    fmt.Printf("  --help, -h     %s\n", helpOptionHelp)
    fmt.Println()
    fmt.Println("Subcommands:")
    fmt.Printf("  forward        %s\n", forwardSubcommandHelp)
    fmt.Printf("  ssh            %s\n", sshSubcommandHelp)
    fmt.Printf("  start          %s\n", startSubcommandHelp)
}

func printForwardSubcommandHelp() {
    fmt.Println("Usage:")
    fmt.Println("  bastion-pod-ctl forward [options]")
    fmt.Println()
    fmt.Println("bastion-pod-ctl forward creates a secure tcp tunnel to an instance in a private network by passing")
    fmt.Println("traffic through a created bastion pod")
    fmt.Println()
    fmt.Println("Options:")
    fmt.Printf("  -h, --help            %s\n", helpOptionHelp)
    fmt.Printf("  -k, --kubeconfig      %s\n", kubeConfigOptionHelp)
    fmt.Printf("  -l, --local-port      %s\n", forwardLocalPortHelp)
    fmt.Printf("  -p, --remote-port     %s\n", forwardRemotePortHelp)
    fmt.Printf("  -r, --remote          %s\n", remoteOptionHelp)
    fmt.Printf("  -v, --verbose         %s\n", verboseOptionHelp)
}

func printSshSubcommandHelp() {
    fmt.Println("Usage:")
    fmt.Println("  bastion-pod-ctl ssh [options]")
    fmt.Println()
    fmt.Println("bastion-pod-ctl ssh creates a secure shell to an instance in a private network through a tunnel created")
    fmt.Println("by passing traffic through a created bastion pod")
    fmt.Println()
    fmt.Println("Options:")
    fmt.Printf("  -h, --help            %s\n", helpOptionHelp)
    fmt.Printf("  -k, --kubeconfig      %s\n", kubeConfigOptionHelp)
    fmt.Printf("  -p, --remote-port     %s\n", sshRemotePortHelp)
    fmt.Printf("  -r, --remote          %s\n", remoteOptionHelp)
    fmt.Printf("  -v, --verbose         %s\n", verboseOptionHelp)
}

func printStartSubcommandHelp() {
    fmt.Println("Usage:")
    fmt.Println("  bastion-pod-ctl start [options]")
    fmt.Println()
    fmt.Println("bastion-pod-ctl start starts up a bastion pod in the specified cluster and then exits, leaving it running.")
    fmt.Println("This should mainly be used for debugging only")
    fmt.Println()
    fmt.Println("Options:")
    fmt.Printf("  -h, --help            %s\n", helpOptionHelp)
    fmt.Printf("  -k, --kubeconfig      %s\n", kubeConfigOptionHelp)
    fmt.Printf("  -v, --verbose         %s\n", verboseOptionHelp)
}

func getForwardFlagSet(options *bastionPodOptions) *flag.FlagSet {
    forwardCommand := flag.NewFlagSet("forward", flag.ExitOnError)
    forwardCommand.BoolVarP(&options.help, "help", "h", false, helpOptionHelp)
    forwardCommand.StringVarP(&options.kubeConfigFile, "kubeconfig", "k", defaultKubeConfigPath, kubeConfigOptionHelp)
    forwardCommand.Int32VarP(&options.localPort, "local-port", "l", -1, forwardLocalPortHelp)
    forwardCommand.Int32VarP(&options.remotePort, "remote-port", "p", -1, forwardRemotePortHelp)
    forwardCommand.StringVarP(&options.remoteHost, "remote", "r", "", remoteOptionHelp)
    forwardCommand.BoolVarP(&options.verbose, "verbose", "v", false, verboseOptionHelp)
    return forwardCommand
}

func getSshFlagSet(options *bastionPodOptions) *flag.FlagSet {
    sshCommand := flag.NewFlagSet("ssh", flag.ExitOnError)
    sshCommand.BoolVarP(&options.help, "help", "h", false, helpOptionHelp)
    sshCommand.StringVarP(&options.kubeConfigFile, "kubeconfig", "k", defaultKubeConfigPath, kubeConfigOptionHelp)
    sshCommand.Int32VarP(&options.remotePort, "remote-port", "p", 22, sshRemotePortHelp)
    sshCommand.StringVarP(&options.remoteHost, "remote", "r", "", remoteOptionHelp)
    sshCommand.BoolVarP(&options.verbose, "verbose", "v", false, verboseOptionHelp)
    return sshCommand
}

func getStartFlagSet(options *bastionPodOptions) *flag.FlagSet {
    startCommand := flag.NewFlagSet("start", flag.ExitOnError)
    startCommand.BoolVarP(&options.help, "help", "h", false, helpOptionHelp)
    startCommand.StringVarP(&options.kubeConfigFile, "kubeconfig", "k", defaultKubeConfigPath, kubeConfigOptionHelp)
    startCommand.BoolVarP(&options.verbose, "verbose", "v", false, verboseOptionHelp)
    return startCommand
}

func checkForwardOptions(options *bastionPodOptions) {
    if options.remoteHost == "" {
        log.Println(remoteHostRequiredMsg)
        printForwardSubcommandHelp()
        os.Exit(1)
    }

    if options.remotePort < 0 {
        log.Println(remotePortRequiredMsg)
        printForwardSubcommandHelp()
        os.Exit(1)
    }

    if options.localPort < 0 {
        options.localPort = options.remotePort
    }
}

func checkSshOptions(options *bastionPodOptions) {
    if options.remoteHost == "" {
        log.Println(remoteHostRequiredMsg)
        printSshSubcommandHelp()
        os.Exit(1)
    }
}

func parseCommandLine() (bastionPodOptions, string) {
    options := bastionPodOptions{}
    forwardFlagSet := getForwardFlagSet(&options)
    sshFlagSet := getSshFlagSet(&options)
    startFlagSet := getStartFlagSet(&options)
    subcommand := os.Args[1]

    switch subcommand {
        case "--help", "-h":
            printBaseCommandHelp()
            os.Exit(0)
        case "forward":
            forwardFlagSet.Parse(os.Args[2:])
        case "ssh":
            sshFlagSet.Parse(os.Args[2:])
        case "start":
            startFlagSet.Parse(os.Args[2:])
        default:
            log.Printf("%q is not valid command.\n", os.Args[1])
            printBaseCommandHelp()
            os.Exit(1)
    }

    if options.help {
        helpForSubcommand[subcommand]()
        os.Exit(0)
    }

    checkOptionsForSubcommand[subcommand](&options)
    return options, subcommand
}
