FROM golang:1.26.9 AS builder

WORKDIR /app

COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/. .

RUN CGO_ENABLED=0 GOOS=linux go build -o source-chat-relay .


FROM gcr.io/distroless/static

COPY --from=builder /app/source-chat-relay /source-chat-relay
COPY --from=builder /app/config.toml.example /config.toml.example

ENTRYPOINT [ "/source-chat-relay" ]

