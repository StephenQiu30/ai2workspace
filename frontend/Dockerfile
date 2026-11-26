FROM golang:1.26.5-alpine AS schema
ARG GOPROXY="https://proxy.golang.org|https://goproxy.cn|direct"
WORKDIR /app
COPY backend/go.mod backend/go.sum ./backend/
COPY backend/cmd ./backend/cmd
COPY backend/internal ./backend/internal
RUN cd backend && GOWORK=off go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -d cmd/ai2workspace,internal -g main.go --parseInternal -o api --outputTypes go,json --quiet

FROM node:24.19.0-alpine AS build
RUN corepack enable && corepack prepare pnpm@12.4.2 --activate
WORKDIR /app/frontend
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ ./
COPY --from=schema /app/backend/api/swagger.json /app/backend/api/swagger.json
ARG API_ORIGIN=http://backend:3123
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
COPY --from=build --chown=node:node /app/frontend/public ./public
USER node
EXPOSE 3121
CMD ["node", "server.js"]
