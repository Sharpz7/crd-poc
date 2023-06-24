FROM ubuntu:focal
WORKDIR /opt
COPY ./bin/crd-poc .
CMD ["./crd-poc", "--tls-cert", "/etc/opt/tls.crt", "--tls-key", "/etc/opt/tls.key"]