FROM golang:1.22-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/kupool-client ./cmd/kupool-client

FROM alpine:3.20
WORKDIR /app
RUN adduser -D -h /app appuser
COPY --from=builder /out/kupool-client /app/kupool-client
USER appuser
ENTRYPOINT ["/app/kupool-client"]

