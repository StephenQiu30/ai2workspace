FROM golang:1.26.5-alpine AS build
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app
COPY backend/services/identity/go.mod backend/services/identity/go.sum ./
RUN go mod download
COPY backend/services/identity/ ./
RUN go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go --parseInternal -o api --outputTypes go,json --quiet
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S identity && adduser -S -G identity identity
WORKDIR /app
COPY --from=build /out/server ./server
USER identity
EXPOSE 3123
ENTRYPOINT ["./server"]
