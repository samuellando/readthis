FROM golang:1.25 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/readthis ./cmd/readthis
RUN mkdir -p /out/data

FROM alpine:3.23

ENV READTHIS_DATA_PATH=/data/links.json
WORKDIR /
COPY --from=builder /out/readthis /readthis
COPY --from=builder /out/data /data
RUN addgroup -S readthis && adduser -S readthis -G readthis && chown -R readthis:readthis /data

EXPOSE 8080
USER readthis
ENTRYPOINT ["/readthis"]
