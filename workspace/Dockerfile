FROM node:24.19.0-alpine AS build
RUN corepack enable && corepack prepare pnpm@12.4.2 --activate
WORKDIR /app
COPY workspace/package.json workspace/pnpm-lock.yaml workspace/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY workspace/ ./
ENV NEXT_TELEMETRY_DISABLED=1
RUN pnpm build

FROM node:24.19.0-alpine
WORKDIR /app
RUN npm install -g serve@14.2.4
COPY --from=build --chown=node:node /app/out ./out
ENV PORT=3122
EXPOSE 3122
USER node
CMD ["serve", "out", "--listen", "3122"]
