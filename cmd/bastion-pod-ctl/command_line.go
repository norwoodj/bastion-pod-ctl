package main

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const forwardLocalPortHelp = "The local port of the tunnel proxying traffic to the private remote host (default = ${remote-port})"
const kubeConfigOptionHelp = "Supply path to a kubeconfig file to use in authenticating to the kubernetes cluster API"
const kubeContextOptionHelp = "Name of kubernetes context to use"
const cpuRequestOptionHelp = "The quantity of CPU to be used in the requests/limits field on bastion pods"
const memoryRequestOptionHelp = "The quantity of memory to be used in the requests/limits field on bastion pods"
const namespaceOptionHelp = "The namespace in which bastion pods should be created"
const sshRemotePortHelp = "The ssh port of the remote host we're tunneling to"
const verboseHelp = "Print verbose output"

const clearSubcommandHelp = "Delete all running bastion pods in the specified namespace"
const forwardSubcommandHelp = "Open a TCP tunnel through the created bastion pod to the specified private remote host"
const listSubcommandHelp = "Print a list of currently running bastion pods"
const sshSubcommandHelp = "SSH through the created bastion pod to the specified private remote host"
const startSubcommandHelp = "For debugging - starts up a bastion pod in the specified cluster and then exits, leaving it running"

const defaultBastionNamespace = "bastion"

var version string


func possibleLogLevels() []string {
	levels := make([]string, 0)

	for _, l := range log.AllLevels {
		levels = append(levels, l.String())
	}

	return levels
}

func newRootCommand() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "bastion-pod-ctl",
		Short: "bastion-pod-ctl is a tool for for forwarding tcp traffic through pods running on kubernetes worker nodes in a private network",
		Version: version,
	}

	logLevelUsage := fmt.Sprintf("Level of logs that should printed, one of (%s)", strings.Join(possibleLogLevels(), ", "))

	rootCmd.PersistentFlags().String("kube-config", "", kubeConfigOptionHelp)
	rootCmd.PersistentFlags().String("kube-context", "", kubeContextOptionHelp)
	rootCmd.PersistentFlags().StringP("log-level", "l", "info", logLevelUsage)
	rootCmd.PersistentFlags().StringP("cpu-request", "c", "5m", cpuRequestOptionHelp)
	rootCmd.PersistentFlags().StringP("memory-request", "m", "16M", memoryRequestOptionHelp)
	rootCmd.PersistentFlags().StringP("namespace", "n", defaultBastionNamespace, namespaceOptionHelp)

	viper.AutomaticEnv()
	viper.SetEnvPrefix("BASTION_POD")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.BindPFlags(rootCmd.PersistentFlags())

	rootCmd.AddCommand(newClearCommand())
	rootCmd.AddCommand(newForwardCommand())
	rootCmd.AddCommand(newListCommand())
	rootCmd.AddCommand(newSshCommand())
	rootCmd.AddCommand(newStartCommand())

	return rootCmd
}

func newForwardCommand() *cobra.Command {
	forwardCommand := &cobra.Command{
		Use:   "forward <remote-host> <remote-port>",
		Args: cobra.ExactArgs(2),
		Short: forwardSubcommandHelp,
		Run: forwardSubcommand,
	}

	forwardCommand.Flags().IntP("local-port", "p", -1, forwardLocalPortHelp)
	viper.BindPFlags(forwardCommand.Flags())
	return forwardCommand
}

func newSshCommand() *cobra.Command {
	sshCommand := &cobra.Command{
		Use:   "ssh <remote-host>",
		Args: cobra.ExactArgs(1),
		Short: sshSubcommandHelp,
		Run: sshSubcommand,
	}

	sshCommand.Flags().IntP("remote-port", "r", 22, sshRemotePortHelp)
	sshCommand.Flags().BoolP("verbose", "v", false, verboseHelp)
	viper.BindPFlags(sshCommand.Flags())
	return sshCommand
}

func newStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start <remote-host> <remote-port>",
		Args: cobra.ExactArgs(2),
		Short: startSubcommandHelp,
		Run: startSubcommand,
	}
}

func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: listSubcommandHelp,
		Run: listSubcommand,
	}
}

func newClearCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: clearSubcommandHelp,
		Run: clearSubcommand,
	}
}
