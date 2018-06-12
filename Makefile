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
	docker build -t quay.io/kubevirt/device-plugin-$*:latest ./cmd/$(subst -,/,$*)

docker-push: $(patsubst %, docker-push-%, $(DOCKERFILES))

docker-push-%:
	docker push quay.io/kubevirt/device-plugin-$*:latest

docker-local-push: $(patsubst %, docker-local-push-%, $(DOCKERFILES))

docker-local-push-%:
	docker tag quay.io/kubevirt/device-plugin-$*:latest localhost:5000/quay.io/kubevirt/device-plugin-$*
	docker push localhost:5000/quay.io/kubevirt/device-plugin-$*:latest

dep:
	dep ensure -v

clean-dep:
	rm -f ./Gopkg.lock
	rm -rf ./vendor

.PHONY: build test docker-build docker-push docker-local-push dep clean-dep

