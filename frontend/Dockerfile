FROM golang:1.26.5-alpine AS schema
WORKDIR /app
COPY backend/services/ai2workspace/go.mod backend/services/ai2workspace/go.sum ./
RUN go mod download
COPY backend/services/ai2workspace/ ./
RUN go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go --parseInternal -o internal/apidocs --outputTypes go,json --quiet

FROM node:24.19.0-alpine AS build
RUN corepack enable && corepack prepare pnpm@12.4.2 --activate
WORKDIR /app
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
COPY --from=schema /app/internal/apidocs/swagger.json /contracts/swagger.json
ENV OPENAPI_SCHEMA_PATH=/contracts/swagger.json
ENV NEXT_TELEMETRY_DISABLED=1
RUN pnpm openapi2ts && pnpm build

FROM node:24.19.0-alpine
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV HOSTNAME=0.0.0.0
ENV PORT=3121
WORKDIR /app
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static
USER node
EXPOSE 3121
CMD ["node", "server.js"]
