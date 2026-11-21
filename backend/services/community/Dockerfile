FROM golang:1.26.5-alpine AS build
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app
COPY backend/services/community/go.mod backend/services/community/go.sum ./
RUN go mod download
COPY backend/services/community/ ./
RUN go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go --parseInternal -o api --outputTypes go,json --quiet
RUN CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata && addgroup -S community && adduser -S -G community community
WORKDIR /app
COPY --from=build /out/server ./server
USER community
EXPOSE 3126
ENTRYPOINT ["./server"]
