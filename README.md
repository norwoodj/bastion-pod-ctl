bastion-pod-ctl
===============
[![Go Report Card](https://goreportcard.com/badge/github.com/norwoodj/bastion-pod-ctl)](https://goreportcard.com/report/github.com/norwoodj/bastion-pod-ctl)

A tool for creating tcp tunnels into a private network via a Pod running on a kubernetes worker node
in that private network. This script has commands for ssh-ing into an instance through a tunnel
through the pod or simply opening a local port for you to connect to other applications through.

## Installation
`bastion-pod-ctl` can be installed using [homebrew](https://brew.sh/):

```
brew install norwoodj/tap/bastion-pod-ctl
```

This will download and install the [latest release](https://github.com/norwoodj/bastion-pod-ctl/releases/latest)
of the tool.

To build from source in this repository:

```
cd cmd/bastion-pod-ctl
go build
```

## How it works
A [Bastion Host](https://en.wikipedia.org/wiki/Bastion_host) is a server that typically runs a single
application like sshd or a proxy server, and acts as a gateway into a private network. This tool
replaces the server in this setup with a pod running on a worker node. This pod acts as the Bastion
Host in this situation, and can proxy traffic into the private network in the same way:
```
                       ______________________________________________________________
                      | Provider Network   ________________________________________  |
                      |                   | Private Subnet                         | |
 _____________        |  __________       |  _________        ___________________  | |
|             |       | |          |      | | Bastion |      | Database or       | | |
|             | https | |          |  tcp | | Pod on  | tcp  | other worker      | | |
| workstation | ======> | Kube API | =====> | Worker  | ===> | node or other     | | |
|             |       | |          |      | | Node    |      | internal service  | | |
|_____________|       | |__________|      | |_________|      |___________________| | |
                      |                   |________________________________________| |
                      |______________________________________________________________|
```

This works in practice by starting an `alpine/socat` pod that forwards tcp on port 8080
then opening a port-forward tunnel locally to forward local traffic to that pod. This assumes
that the pods run on worker nodes in a private network that's not accessible from the public
internet. By forwarding a port to the pod in the private subnet, the pod acts as a proxy
into the private network, enabling one to ssh into private instances or connect to
private databases in emergency situations.


## Examples
These examples will use whatever current context you have set in your configured kubeconfig file
to authenticate to the cluster, same as kubectl. Keep in mind that to run the ssh example, you'll
need to configure whatever SSH key you use to authenticate to the instance in question to work for
`localhost`, as the utility forks an ssh process that connects to the tunnel port on your local
machine. Something like:
```
##
# Bastion Pod
##
Host localhost
  IdentityFile ~/.ssh/ec2.id_rsa
```

SSH into a private instance via a pod running on a worker node in that same network
```bash
bastion-pod-ctl ssh ip-10-0-23-23.us-west-2.compute.internal
```

Forward traffic through a pod on a worker node to a private postgres database
```bash
bastion-pod-ctl forward primary.postgres.cool-application.com 5432
```

Once the above is running you can connect with:
```bash
docker run --rm -it \
    --entrypoint psql \
    postgres:10.4 \
        --host=docker.for.mac.host.internal \
        --username=cool-app-ro \
        --dbname=cool-app
```
