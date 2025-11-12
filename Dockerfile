# syntax=docker/dockerfile:1
FROM golang:1.25 AS builder
ARG GOPROXY=https://proxy.golang.org,direct
ARG APP=server
ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    GOTOOLCHAIN=local \
    GOPROXY=${GOPROXY} \
    GOFLAGS=-buildvcs=false
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/kupool-${APP} ./cmd/kupool-${APP}

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
ARG APP=server
COPY --from=builder /out/kupool-${APP} /usr/local/bin/kupool-app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/kupool-app"]
