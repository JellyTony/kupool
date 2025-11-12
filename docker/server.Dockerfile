FROM golang:1.22-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/kupool-server ./cmd/kupool-server

FROM alpine:3.20
WORKDIR /app
RUN adduser -D -h /app appuser
COPY --from=builder /out/kupool-server /app/kupool-server
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/kupool-server"]

