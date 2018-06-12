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

build: $(patsubst %, build-%, $(PLUGINS))

build-%:
	cd cmd/$(subst -,/,$*) && go install -v

test:
	go test ./cmd/... ./pkg/...

test-%:
	go test ./$(subst -,/,$*)/...

docker-build: $(patsubst %, docker-build-%, $(DOCKERFILES))

docker-build-%:
	@cp ${GOPATH}/bin/${notdir $(subst -,/,$*)} ./cmd/$(subst -,/,$*)
	docker build -t ${REGISTRY}/device-plugin-$*:latest ./cmd/$(subst -,/,$*)

docker-push: $(patsubst %, docker-push-%, $(DOCKERFILES))

docker-push-%:
	docker tag ${REGISTRY}/device-plugin-$*:latest ${REGISTRY}/device-plugin-$*
	docker push ${REGISTRY}/device-plugin-$*:latest

dep:
	dep ensure -v

clean-dep:
	rm -f ./Gopkg.lock
	rm -rf ./vendor

.PHONY: build test docker-build docker-push docker-local-push dep clean-dep

