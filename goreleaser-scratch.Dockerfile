FROM gcr.io/distroless/cc-debian12:latest
COPY validator-tools* /validator-tools
ENTRYPOINT ["/validator-tools"]
