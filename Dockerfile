FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY *.go ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o table_size_exporter .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/table_size_exporter /table_size_exporter

EXPOSE 9100

ENTRYPOINT ["/table_size_exporter"]
