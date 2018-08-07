FROM golang:1.10 AS builder
WORKDIR /go/src/github.com/kubevirt/kubernetes-device-plugins/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bridge github.com/kubevirt/kubernetes-device-plugins/cmd/network/bridge

FROM fedora:27
RUN dnf install -y iproute
COPY --from=builder /bridge /bridge
ENTRYPOINT [ "./bridge", "-v", "3", "-logtostderr"]
