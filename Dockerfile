# --- этап 1: сборка фронтенда -------------------------------------------------
FROM node:22-alpine AS webbuild
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- этап 2: сборка Go-бинарников ---------------------------------------------
FROM golang:1.25-alpine AS gobuild
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/collector ./cmd/collector \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/vapid ./cmd/tools/vapid

# --- этап 3: рантайм ------------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata curl
WORKDIR /app
COPY --from=gobuild /out/ /app/bin/
COPY --from=webbuild /web/dist /app/web/dist
ENV WEB_DIR=/app/web/dist
EXPOSE 8080
CMD ["/app/bin/server"]
