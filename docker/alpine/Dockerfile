FROM golang:1.24-alpine3.23
WORKDIR /lazyjournal

COPY go.mod go.sum ./
RUN go mod download

COPY main.go .
ARG TARGETOS TARGETARCH
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /bin/lazyjournal && chmod 755 /bin/lazyjournal

COPY config.yml /bin/

ENTRYPOINT ["lazyjournal"]