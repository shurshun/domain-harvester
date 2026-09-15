# Consumes the pre-built binary goreleaser drops into the build context.
# For a self-contained `docker build .` from source, see Dockerfile.local.
FROM gcr.io/distroless/static:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

COPY domain-harvester /domain-harvester

USER nonroot:nonroot

ENTRYPOINT ["/domain-harvester"]
