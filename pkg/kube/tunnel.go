package kube

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// Tunnel describes a ssh-like tunnel to a kubernetes pod
type Tunnel struct {
	Local     int
	Remote    int
	Namespace string
	PodName   string
	Out       io.Writer
	stopChan  chan struct{}
	readyChan chan struct{}
	config    *rest.Config
	client    rest.Interface
}

// NewTunnel creates a new tunnel
func NewTunnel(client rest.Interface, config *rest.Config, namespace, podName string, remote int) *Tunnel {
	return &Tunnel{
		config:    config,
		client:    client,
		Namespace: namespace,
		PodName:   podName,
		Remote:    remote,
		stopChan:  make(chan struct{}, 1),
		readyChan: make(chan struct{}, 1),
		Out:       ioutil.Discard,
	}
}

// Close disconnects a tunnel connection
func (t *Tunnel) Close() {
	close(t.stopChan)
}

// ForwardPort opens a tunnel to a kubernetes pod
func (t *Tunnel) ForwardPort(localPort int) error {
	// Build a url to the portforward endpoint
	// example: http://localhost:8080/api/v1/namespaces/helm/pods/tiller-deploy-9itlq/portforward
	u := t.client.Post().
		Resource("pods").
		Namespace(t.Namespace).
		Name(t.PodName).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(t.config)

	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", u)
	t.Local = localPort
	ports := []string{fmt.Sprintf("%d:%d", t.Local, t.Remote)}

	pf, err := portforward.New(dialer, ports, t.stopChan, t.readyChan, t.Out, t.Out)
	if err != nil {
		return err
	}

	errChan := make(chan error)
	go func() {
		errChan <- pf.ForwardPorts()
	}()

	select {
	case err = <-errChan:
		return fmt.Errorf("forwarding ports: %v", err)
	case <-pf.Ready:
		return nil
	}
}
