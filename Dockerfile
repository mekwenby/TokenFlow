FROM golang:1.21-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/tokenflow ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/tokenflow /app/tokenflow

ENV GATEWAY_ADDR=:8019
ENV GATEWAY_DATA_DIR=/app/data

EXPOSE 8019
VOLUME ["/app/data"]

CMD ["/app/tokenflow"]
