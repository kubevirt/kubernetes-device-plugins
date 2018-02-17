PLUGINS = $(sort \
		  $(subst /, -, \
		  $(patsubst cmd/%/, %, \
		  $(dir \
		  $(shell find cmd/ -type f -name '*.go')))))

DOCKERFILES = $(sort \
			  $(subst /, -, \
			  $(patsubst cmd/%/, %, \
			  $(dir \
			  $(shell find cmd/ -type f -name 'Dockerfile')))))

build: $(patsubst %, build-%, $(PLUGINS))

build-%:
	cd cmd/$(subst -,/,$*) && go build

test:
	go test ./cmd/... ./pkg/...

test-%:
	go test ./$(subst -,/,$*)/...

docker-build: $(patsubst %, docker-build-%, $(DOCKERFILES))

docker-build-%:
	docker build -t quay.io/kubevirt/device-plugin-$*:latest cmd/$(subst -,/,$*)

docker-push: $(patsubst %, docker-push-%, $(DOCKERFILES))

docker-push-%:
	docker push quay.io/kubevirt/device-plugin-$*:latest

.PHONY: build
