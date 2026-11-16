FROM golang:1.26.5-alpine AS build
WORKDIR /app
COPY backend/services/ai2workspace/go.mod backend/services/ai2workspace/go.sum ./
RUN go mod download
COPY backend/services/ai2workspace/ ./
RUN go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go --parseInternal -o internal/apidocs --outputTypes go,json --quiet
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S ai2workspace && adduser -S -G ai2workspace ai2workspace
WORKDIR /app
COPY --from=build /out/server ./server
USER ai2workspace
EXPOSE 8080
ENTRYPOINT ["./server"]
