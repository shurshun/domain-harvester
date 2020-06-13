FROM alpine:latest

COPY domain-harvester /

ENTRYPOINT ["/domain-harvester"]
