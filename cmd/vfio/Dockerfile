FROM golang:1.10 AS builder
WORKDIR /go/src/github.com/kubevirt/kubernetes-device-plugins/
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /vfio github.com/kubevirt/kubernetes-device-plugins/cmd/vfio

FROM fedora:27
RUN yum -y install kmod
COPY --from=builder /vfio /vfio
ENTRYPOINT ["/vfio"]
