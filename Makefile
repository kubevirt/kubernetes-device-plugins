PLUGINS = $(sort \
		  $(subst /, -, \
		  $(patsubst cmd/%/, %, \
		  $(dir \
		  $(shell find cmd/ -type f -name '*.go')))))

build: $(patsubst %, build-%, $(PLUGINS))

build-%:
	cd cmd/$(subst -,/,$*) && go build

test:
	go test ./cmd/... ./pkg/...

test-%:
	go test ./$(subst -,/,$*)/...

.PHONY: build
