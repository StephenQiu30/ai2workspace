FROM golang:1.26.5-alpine AS build
WORKDIR /app
ENV GOWORK=off
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/cmd ./cmd
COPY backend/internal ./internal
RUN go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -d cmd/server,internal -g main.go --parseInternal -o api --outputTypes go,json --quiet
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -S studio && adduser -S -G studio studio
COPY --from=build /out/server /usr/local/bin/studio
USER studio
EXPOSE 3128
ENTRYPOINT ["/usr/local/bin/studio"]
