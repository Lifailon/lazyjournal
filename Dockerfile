# Build source code for different architectures
FROM golang:1.23-alpine3.20 AS build
WORKDIR /lazyjournal
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /bin/lazyjournal
# Download ttyd
RUN apk add -U -q --progress --no-cache curl
RUN ARCH=$(case ${TARGETARCH} in \
    "amd64") echo "x86_64" ;; \
    "arm64") echo "aarch64" ;; \
    "arm") echo "armhf" ;; \
    *) echo "${TARGETARCH}" ;; \
    esac) && \
    curl -fsSL "https://github.com/tsl0922/ttyd/releases/download/1.7.7/ttyd.${ARCH}" -o /bin/ttyd && \
    chmod +x /bin/ttyd

# Build docker cli
FROM golang:1.23-alpine3.20 AS docker-build
RUN apk add -U -q --progress --no-cache git bash coreutils gcc musl-dev
WORKDIR /go/src/github.com/docker/cli
RUN git clone --branch v27.0.3 --single-branch --depth 1 https://github.com/docker/cli .
ARG TARGETOS TARGETARCH
ENV CGO_ENABLED=0
ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}
ENV DISABLE_WARN_OUTSIDE_CONTAINER=1
RUN ./scripts/build/binary
RUN rm build/docker && mv build/docker-${TARGETOS}-${TARGETARCH} build/docker

# Final image - используем multi-arch базовый образ
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