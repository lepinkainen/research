FROM golang:1.22-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
COPY static/ ./static/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o anki .

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
RUN mkdir -p /data && chown nobody:nobody /data
WORKDIR /app
COPY --from=builder /build/anki .

EXPOSE 8080
VOLUME ["/data"]
ENV DB_PATH=/data/anki.db
ENV PORT=8080

USER nobody
ENTRYPOINT ["./anki"]
