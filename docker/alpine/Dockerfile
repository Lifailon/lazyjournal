FROM golang:1.27.0-alpine3.24 AS build-app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS TARGETARCH
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/lazyjournal

FROM alpine:3.24

RUN apk add --no-cache docker-cli docker-cli-compose kubectl ca-certificates

WORKDIR /lazyjournal
COPY config.yml .
COPY --chmod=755 --from=build-app /bin/lazyjournal /bin/

ENTRYPOINT ["lazyjournal"]