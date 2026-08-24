# Consumes the pre-built binary goreleaser drops into the build context.
# For a self-contained `docker build .` from source, see Dockerfile.local.
FROM gcr.io/distroless/static:nonroot@sha256:1c2c046bc09ed40fad370b599a0b1ae7987f55b01e247cf27a7c27cd97e5bbc7

COPY domain-harvester /domain-harvester

USER nonroot:nonroot

ENTRYPOINT ["/domain-harvester"]
