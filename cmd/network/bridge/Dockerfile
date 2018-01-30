FROM fedora:27

RUN dnf install -y iproute

COPY bridge /bridge

ENTRYPOINT [ "./bridge", "-v", "3", "-logtostderr"]
