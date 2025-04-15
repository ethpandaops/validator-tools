FROM golang:1.24 AS builder
WORKDIR /src
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN go build -o /bin/app .

FROM ubuntu:latest
RUN apt-get update && apt-get -y upgrade && apt-get install -y --no-install-recommends \
  libssl-dev \
  ca-certificates \
  jq \
  curl \
  wget \
  zsh \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

RUN chsh -s /bin/zsh

RUN wget https://github.com/wealdtech/ethdo/releases/download/v1.37.3/ethdo-1.37.3-linux-amd64.tar.gz \
  && tar xzf ethdo-1.37.3-linux-amd64.tar.gz \
  && mv ethdo /usr/bin/ \
  && rm ethdo-1.37.3-linux-amd64.tar.gz

COPY --from=builder /bin/app /validator-tools
ENTRYPOINT ["/validator-tools"]
