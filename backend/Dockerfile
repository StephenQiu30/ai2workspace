FROM golang:1.26.5-alpine AS build
ARG GOPROXY="https://proxy.golang.org|https://goproxy.cn|direct"
WORKDIR /app
ENV GOWORK=off
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/cmd ./cmd
COPY backend/internal ./internal
RUN go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -d cmd/ai2workspace -g main.go --parseInternal --parseDependencyLevel 3 -o api --outputTypes go,json --quiet
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ai2workspace ./cmd/ai2workspace

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S app && adduser -S -G app app
COPY --from=build /out/ai2workspace /usr/local/bin/ai2workspace
USER app
EXPOSE 3123
ENTRYPOINT ["/usr/local/bin/ai2workspace"]
