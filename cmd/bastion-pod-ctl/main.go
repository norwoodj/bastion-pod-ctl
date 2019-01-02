package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/norwoodj/bastion-pod-ctl/pkg/kube"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func setupExitHandlers(kubeClient kubernetes.Interface, bastionPod *v1.Pod, tunnel *kube.Tunnel) chan bool {
    gracefulStop := make(chan os.Signal)
	done := make(chan bool)
    signal.Notify(gracefulStop, syscall.SIGTERM)
    signal.Notify(gracefulStop, syscall.SIGINT)

    go func() {
        sig := <-gracefulStop
        log.Infof("Caught Signal: %+v, exiting gracefully...", sig)
        cleanup(kubeClient, bastionPod, tunnel)
        os.Exit(0)
        close(done)
    }()

	return done
}

func main() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	rootCmd := newRootCommand()

    if err := rootCmd.Execute(); err != nil {
        log.Error(err)
        os.Exit(1)
    }
}
