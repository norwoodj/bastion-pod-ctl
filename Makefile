all: build

deps:
	go get "github.com/phayes/freeport"
	go get "github.com/spf13/pflag"
	go get "k8s.io/api/core/v1"
	go get "k8s.io/apimachinery/pkg/apis/meta/v1"
	go get "k8s.io/client-go/kubernetes"
	go get "k8s.io/client-go/tools/clientcmd"
	go get "k8s.io/client-go/util/homedir"

build:
	go build -ldflags '-extldflags "-static"'

clean:
	go clean

.PHONY: build clean
