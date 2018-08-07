FROM golang:1.10 AS builder
WORKDIR /go/src/github.com/kubevirt/kubernetes-device-plugins/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /kvm github.com/kubevirt/kubernetes-device-plugins/cmd/kvm

FROM fedora:27
COPY --from=builder /kvm /kvm
ENTRYPOINT ["/kvm"]
