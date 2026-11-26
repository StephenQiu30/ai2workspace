FROM golang:1.26.5-alpine AS schema
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app
COPY backend/services/ ./backend/services/
RUN for svc in identity learning commerce community assistant; do \
      (cd "/app/backend/services/$svc" && GOWORK=off go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go --parseInternal -o api --outputTypes go,json --quiet); \
    done
COPY backend/go.mod backend/go.sum ./backend/
COPY backend/cmd ./backend/cmd
COPY backend/internal ./backend/internal
RUN cd backend && GOWORK=off go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -d cmd/server,internal -g main.go --parseInternal -o api --outputTypes go,json --quiet

FROM node:24.19.0-alpine AS build
RUN corepack enable && corepack prepare pnpm@12.4.2 --activate
WORKDIR /app/frontend
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
COPY --from=schema /app/backend/services/identity/api/swagger.json /app/backend/services/identity/api/swagger.json
COPY --from=schema /app/backend/services/learning/api/swagger.json /app/backend/services/learning/api/swagger.json
COPY --from=schema /app/backend/services/commerce/api/swagger.json /app/backend/services/commerce/api/swagger.json
COPY --from=schema /app/backend/services/community/api/swagger.json /app/backend/services/community/api/swagger.json
COPY --from=schema /app/backend/services/assistant/api/swagger.json /app/backend/services/assistant/api/swagger.json
COPY --from=schema /app/backend/api/swagger.json /app/backend/api/swagger.json
ENV NEXT_TELEMETRY_DISABLED=1
RUN pnpm openapi2ts && pnpm build

FROM node:24.19.0-alpine
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
ENV HOSTNAME=0.0.0.0
ENV PORT=3121
WORKDIR /app
COPY --from=build --chown=node:node /app/frontend/.next/standalone ./
COPY --from=build --chown=node:node /app/frontend/.next/static ./.next/static
USER node
EXPOSE 3121
CMD ["node", "server.js"]
