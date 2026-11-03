FROM node:23.7.0-alpine3.21 AS web-build
WORKDIR /src
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci
COPY web ./web
COPY server/internal/webui/embed.go ./server/internal/webui/embed.go
RUN cd web && npm run build

FROM golang:1.27.3-alpine3.24 AS server-build
WORKDIR /src
COPY server ./server
COPY --from=web-build /src/server/internal/webui/dist ./server/internal/webui/dist
RUN cd server && CGO_ENABLED=0 go build -trimpath -o /out/muddys ./cmd/server

FROM alpine:3.24
RUN adduser -D -u 1000 muddys && mkdir -p /data && chown muddys:muddys /data
COPY --from=server-build /out/muddys /usr/local/bin/muddys
USER muddys
EXPOSE 8080
EXPOSE 9090
# Probe the internal listener: the public one answers only its configured hostnames.
HEALTHCHECK --interval=10s --timeout=3s --start-period=20s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:9090/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/muddys", "-addr=:8080", "-data-dir=/data"]
