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
    remoteHost     string
    remotePort     int32
    verbose        bool
}

var defaultKubeConfigPath = filepath.Join(homedir.HomeDir(), ".kube", "config")

const helpOptionHelp = "Print this help menu, then exist"
const remoteOptionHelp = "The remote host to which this will tunnel an ssh connection through a bastion pod"
const sshRemotePortHelp = "The ssh port of the remote host we're tunneling to (default = 22)"
const kubeConfigOptionHelp = "Supply path to a kubeconfig file to use in authenticating to the kubernetes cluster API (default = ~/.kube/config)"
const verboseOptionHelp = "Print verbose output"

const forwardSubcommandHelp = "Open a TCP tunnel through the created bastion pod to the specified private remoteHost"
const sshSubcommandHelp = "SSH through the created bastion pod to the specified private remoteHost"

var helpForSubcommand = map[string]func(){
    "ssh": printSshSubcommandHelp,
}

func printBaseCommandHelp() {
    fmt.Println("Usage:")
    fmt.Println("  bastion-pod-ctl [options] [subcommand]")
    fmt.Println()
    fmt.Println("bastion-pod-ctl creates a pod running a proxy forwarding TCP traffic to a specified address. Subcommands")
    fmt.Println("can be used to ssh through this tunnel or simply to leave the tunnel open for other applications to proxy")
    fmt.Println("traffic to the private address")
    fmt.Println()
    fmt.Println("Options:")
    fmt.Printf("  --help, -h     %s\n", helpOptionHelp)
    fmt.Println()
    fmt.Println("Subcommands:")
    fmt.Printf("  forward        %s\n", forwardSubcommandHelp)
    fmt.Printf("  ssh            %s\n", sshSubcommandHelp)
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
    fmt.Printf("  -r, --remote          %s\n", remoteOptionHelp)
    fmt.Printf("  -p, --remote-port     %s\n", sshRemotePortHelp)
    fmt.Printf("  -v, --verbose         %s\n", verboseOptionHelp)
}

func getSshFlagSet(options *bastionPodOptions) *flag.FlagSet {
    sshCommand := flag.NewFlagSet("ssh", flag.ExitOnError)
    sshCommand.BoolVarP(&options.help, "help", "h", false, helpOptionHelp)
    sshCommand.StringVarP(&options.kubeConfigFile, "kubeconfig", "k", defaultKubeConfigPath, kubeConfigOptionHelp)
    sshCommand.StringVarP(&options.remoteHost, "remote", "r", "", remoteOptionHelp)
    sshCommand.Int32VarP(&options.remotePort, "remote-port", "p", 22, sshRemotePortHelp)
    sshCommand.BoolVarP(&options.verbose, "verbose", "v", false, verboseOptionHelp)
    return sshCommand
}

func parseCommandLine() (bastionPodOptions, string) {
    options := bastionPodOptions{}
    sshFlagSet := getSshFlagSet(&options)
    subcommand := os.Args[1]

    switch subcommand {
        case "--help", "-h":
            printBaseCommandHelp()
            os.Exit(0)
        case "ssh":
            sshFlagSet.Parse(os.Args[2:])
        default:
            log.Printf("%q is not valid command.\n", os.Args[1])
            printBaseCommandHelp()
            os.Exit(1)
    }

    if options.help {
        helpForSubcommand[subcommand]()
        os.Exit(0)
    }

    return options, subcommand
}
