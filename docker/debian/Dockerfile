# build-app lazyjournal and download dependency packages
FROM golang:1.24-alpine3.23 AS build-app
WORKDIR /lazyjournal

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS TARGETARCH
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /bin/lazyjournal

# download ttyd, docker-compose and kubectl
RUN apk add -U -q --progress --no-cache curl jq ca-certificates && \
    ARCH=$(case ${TARGETARCH} in \
    "amd64") echo "x86_64" ;; \
    "arm64") echo "aarch64" ;; \
    *) echo "${TARGETARCH}" ;; \
    esac) && \
    latest=$(curl -sSL https://api.github.com/repos/tsl0922/ttyd/releases/latest | jq -r .tag_name) && \
    curl -fSL "https://github.com/tsl0922/ttyd/releases/download/${latest}/ttyd.${ARCH}" -o /bin/ttyd && \
    latest=$(curl -sSL https://api.github.com/repos/docker/compose/releases/latest | jq -r .tag_name) && \
    curl -fSL "https://github.com/docker/compose/releases/download/${latest}/docker-compose-linux-${ARCH}" -o /bin/docker-compose && \
    latest=$(curl -sSL https://dl.k8s.io/release/stable.txt) && \
    curl -fSL https://dl.k8s.io/release/${latest}/bin/linux/${TARGETARCH}/kubectl -o /bin/kubectl

# build-docker-cli
FROM golang:1.24-alpine3.23 AS build-docker-cli
RUN apk add -U -q --progress --no-cache git bash coreutils gcc musl-dev
WORKDIR /go/src/github.com/docker/cli
RUN git clone --branch v27.0.3 --single-branch --depth 1 https://github.com/docker/cli .
ARG TARGETOS TARGETARCH
ENV CGO_ENABLED=0
ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}
ENV DISABLE_WARN_OUTSIDE_CONTAINER=1
RUN ./scripts/build/binary
RUN mv build/docker-${TARGETOS}-${TARGETARCH} build/docker

# build final image
FROM debian:bookworm-slim
WORKDIR /lazyjournal

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive \
    apt-get install -y --no-install-recommends systemd \
    xz-utils bzip2 gzip && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /usr/local/lib/docker/cli-plugins

COPY --chmod=755 --from=build-docker-cli /go/src/github.com/docker/cli/build/docker /bin/docker
COPY --chmod=755 --from=build-app /bin/docker-compose /usr/local/lib/docker/cli-plugins/docker-compose
COPY --chmod=755 --from=build-app /bin/kubectl /bin/kubectl
COPY --chmod=755 --from=build-app /bin/ttyd /bin/ttyd
COPY --chmod=755 --from=build-app /bin/lazyjournal /bin/lazyjournal
COPY --chmod=755 docker/debian/entrypoint.sh ./
COPY config.yml /bin/

ENTRYPOINT ["/lazyjournal/entrypoint.sh"]