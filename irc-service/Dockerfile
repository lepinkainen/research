FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o irc-service .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    mkdir -p /data && chown nobody:nobody /data

WORKDIR /app
COPY --from=builder /build/irc-service .

EXPOSE 8080
VOLUME ["/data"]
ENV DB_PATH=/data/irc.db
ENV ADDR=:8080

USER nobody
ENTRYPOINT ["./irc-service"]
