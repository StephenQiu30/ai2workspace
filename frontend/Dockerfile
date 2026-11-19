FROM golang:1.26.5-alpine AS schema
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app
COPY backend/services/identity/go.mod backend/services/identity/go.sum ./
RUN go mod download
COPY backend/services/identity/ ./
RUN go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go --parseInternal -o api --outputTypes go,json --quiet

FROM node:24.19.0-alpine AS build
RUN corepack enable && corepack prepare pnpm@12.4.2 --activate
WORKDIR /app
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
COPY --from=schema /app/api/swagger.json /contracts/swagger.json
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
