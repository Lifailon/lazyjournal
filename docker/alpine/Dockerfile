# build-app lazyjournal
FROM golang:1.24-alpine3.23 AS build-app

COPY go.mod go.sum ./
RUN go mod download

COPY main.go .
ARG TARGETOS TARGETARCH
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /bin/lazyjournal

# build final image
FROM alpine:3.23
WORKDIR /lazyjournal

COPY --chmod=755 --from=build-app /bin/lazyjournal .
COPY config.yml .

ENTRYPOINT ["./lazyjournal"]