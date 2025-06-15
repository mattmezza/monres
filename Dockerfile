FROM golang:1.24-alpine AS builder
WORKDIR /app
ENV GO111MODULE=on
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o ./monres ./cmd/monres

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/monres .
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/app/monres"]
CMD ["-config", "/app/config.yaml"]
