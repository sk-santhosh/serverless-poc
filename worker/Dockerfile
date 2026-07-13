# syntax=docker/dockerfile:1

# --- Build stage -------------------------------------------------------
FROM node:20-alpine AS builder
WORKDIR /app

COPY package.json package-lock.json* ./
RUN npm ci

COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# --- Runtime stage -------------------------------------------------------
FROM node:20-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production

RUN addgroup --system --gid 1001 worker \
    && adduser --system --uid 1001 worker

COPY package.json package-lock.json* ./
RUN npm ci --omit=dev

COPY --from=builder /app/dist ./dist

USER worker

# This is a single-shot Job container: it runs once, processes at most one
# message, and exits. No EXPOSE / long-running server here.
CMD ["node", "dist/index.js"]
