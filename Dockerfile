FROM golang:1.27.3-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/projectflow-api ./cmd/api \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/projectflow-worker ./cmd/worker \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/projectflow-db ./cmd/dbsetup

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 appuser

COPY --from=builder /out/projectflow-api /usr/local/bin/projectflow-api
COPY --from=builder /out/projectflow-worker /usr/local/bin/projectflow-worker
COPY --from=builder /out/projectflow-db /usr/local/bin/projectflow-db

USER appuser
EXPOSE 4000
STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/projectflow-api"]
