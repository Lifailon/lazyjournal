# Build source code and download dependencies
FROM golang:1.23-alpine3.20 AS build

WORKDIR /lazyjournal
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build for different architectures
ARG TARGETOS TARGETARCH
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /bin/lazyjournal

RUN apk add -U -q --progress --no-cache curl
RUN curl -fsSL https://github.com/tsl0922/ttyd/releases/download/1.7.7/ttyd.x86_64 -o /bin/ttyd
# RUN curl -fsSL https://github.com/containers/podman/releases/latest/download/podman-remote-static-linux_amd64.tar.gz | tar xz -C /
# RUN latest=$(curl -sL https://dl.k8s.io/release/stable.txt) && \
#     curl -fsSL https://cdn.dl.k8s.io/release/$latest/bin/linux/amd64/kubectl -o /bin/kubectl

# Build docker cli
FROM golang:1.23-alpine3.20 AS docker-build
RUN apk add -U -q --progress --no-cache git bash coreutils gcc musl-dev
WORKDIR /go/src/github.com/docker/cli
RUN git clone --branch v27.0.3 --single-branch --depth 1 https://github.com/docker/cli .
ENV CGO_ENABLED=0
ENV GOARCH=amd64
ENV DISABLE_WARN_OUTSIDE_CONTAINER=1
RUN ./scripts/build/binary
RUN rm build/docker && mv build/docker-linux-* build/docker

# Final image with systemd
FROM debian:bookworm-slim
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive \
    apt-get install -y --no-install-recommends systemd \
    xz-utils bzip2 gzip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

COPY --from=build /bin/lazyjournal /bin/lazyjournal
COPY --from=build /bin/ttyd /bin/ttyd
# COPY --from=build /bin/podman-remote-static-linux_amd64 /bin/podman
# COPY --from=build /bin/kubectl /bin/kubectl
COPY --from=docker-build /go/src/github.com/docker/cli/build/docker /bin/docker

WORKDIR /lazyjournal
COPY config.yml entrypoint.sh ./

RUN chmod +x /bin/lazyjournal /bin/ttyd /bin/docker /lazyjournal/entrypoint.sh

ENTRYPOINT ["/lazyjournal/entrypoint.sh"]