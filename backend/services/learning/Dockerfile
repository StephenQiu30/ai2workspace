FROM golang:1.26.5-alpine AS builder

WORKDIR /src
COPY . .

WORKDIR /src/backend/services/learning
RUN go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go --parseInternal -o api --outputTypes go,json --quiet
RUN CGO_ENABLED=0 GOWORK=off go build -ldflags="-s -w" -o /bin/server ./cmd/server

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /bin/server /app/server
USER 65534:65534
EXPOSE 3124
ENTRYPOINT ["/app/server"]
