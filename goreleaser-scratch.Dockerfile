FROM debian:latest AS builder
RUN apt-get update && apt-get install -y --no-install-recommends \
  jq \
  curl \
  wget \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

RUN wget https://github.com/wealdtech/ethdo/releases/download/v1.37.0/ethdo-1.37.0-linux-amd64.tar.gz \
  && tar xzf ethdo-1.37.0-linux-amd64.tar.gz \
  && mv ethdo /usr/bin/ \
  && rm ethdo-1.37.0-linux-amd64.tar.gz

FROM gcr.io/distroless/cc-debian12:latest
COPY --from=builder /usr/bin/jq /usr/bin/jq
COPY --from=builder /usr/bin/curl /usr/bin/curl
COPY --from=builder /usr/bin/wget /usr/bin/wget
COPY --from=builder /usr/bin/ethdo /usr/bin/ethdo
COPY validator-tools* /validator-tools
ENTRYPOINT ["/validator-tools"]
