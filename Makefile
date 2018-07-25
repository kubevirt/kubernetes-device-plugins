REGISTRY ?= quay.io/kubevirt

PLUGINS = $(sort \
		  $(subst /,-,\
		  $(patsubst cmd/%/,%,\
		  $(dir \
		  $(shell find cmd/ -type f -name '*.go')))))

DOCKERFILES = $(sort \
			  $(subst /,-,\
			  $(patsubst cmd/%/,%,\
			  $(dir \
			  $(shell find cmd/ -type f -name 'Dockerfile')))))

all: build

build: format $(patsubst %, build-%, $(PLUGINS))

build-%:
	cd cmd/$(subst -,/,$*) && go fmt && go vet && go install -v

format:
	go fmt ./pkg/...
	go vet ./pkg/...

test:
	go test ./cmd/... ./pkg/...

test-%:
	go test ./$(subst -,/,$*)/...

functest:
	pytest tests

docker-build: $(patsubst %, docker-build-%, $(DOCKERFILES))

docker-build-%:
	@cp ${GOPATH}/bin/${notdir $(subst -,/,$*)} ./cmd/$(subst -,/,$*)
	docker build -t ${REGISTRY}/device-plugin-$*:latest ./cmd/$(subst -,/,$*)

docker-push: $(patsubst %, docker-push-%, $(DOCKERFILES))

docker-push-%:
	docker push ${REGISTRY}/device-plugin-$*:latest

dep:
	dep ensure -v

clean-dep:
	rm -f ./Gopkg.lock
	rm -rf ./vendor

cluster-up:
	./cluster/up.sh

cluster-down:
	./cluster/down.sh

cluster-sync: $(patsubst %, cluster-sync-%, $(DOCKERFILES))

cluster-sync-%:
	./cluster/build.sh $*
	./cluster/sync.sh $*

.PHONY: format build test docker-build docker-push docker-local-push dep clean-dep

