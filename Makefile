.PHONY: all
all: bastion-pod-ctl

bastion-pod-ctl:
	(cd cmd/bastion-pod-ctl && go build)
	mv cmd/bastion-pod-ctl/bastion-pod-ctl  .

.PHONY: clean
clean:
	rm bastion-pod-ctl
