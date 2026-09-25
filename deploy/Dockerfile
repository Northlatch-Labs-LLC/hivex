# hivex engine image — multi-stage: bun web build → go build → distroless.
#
# Build from the REPO ROOT (the go:embed at the root needs web/dist and
# templates/ inside the Go build stage):
#
#   docker build -f deploy/Dockerfile -t hivex .
#
# The engine binds its HTTP listeners to 127.0.0.1 by design; expose it to
# the internet ONLY through the Caddy sidecar in deploy/docker-compose.yml,
# which shares this container's network namespace, and list your domain in
# HIVEX_WEB_PUBLIC_HOSTS (see docker-compose.yml).
#
# NOTE: this image ships the engine + web UI. Bot turn execution shells out
# to LLM runtime CLIs (claude/codex/opencode) which are NOT bundled here;
# deployments that need in-container bots must extend the final stage.

# ---- Stage 1: web UI bundle (bun) ------------------------------------
FROM oven/bun:1 AS webbuild
WORKDIR /src/web
# Layers: manifests first so dependency install caches independently of src.
COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile
COPY web/ ./
# Produces web/dist (vite) — embedded into the binary in stage 2.
RUN bun run build

# ---- Stage 2: go build ------------------------------------------------
FROM golang:1.26-bookworm AS gobuild
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux
# go.mod pins `toolchain go1.26.6`; the golang:1.26 image satisfies it.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The root embed.go (`//go:embed all:web/dist`) needs the bundle from stage 1.
COPY --from=webbuild /src/web/dist ./web/dist
RUN go build -trimpath -ldflags="-s -w" -o /out/hivex ./cmd/hivex
# /data is the runtime home the nonroot user must be able to write to.
RUN mkdir -p /data && chown 65532:65532 /data

# ---- Stage 3: minimal runtime ----------------------------------------
# static (CGO_ENABLED=0; the sqlite driver is pure-Go modernc.org/sqlite),
# nonroot user 65532. Includes ca-certificates; no shell, no package manager.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=gobuild /out/hivex /hivex
COPY --from=gobuild --chown=65532:65532 /data /data
ENV HIVEX_RUNTIME_HOME=/data
WORKDIR /data
# Broker API 7890, web UI 7891 — both bind 127.0.0.1 inside the container.
EXPOSE 7890 7891
# runWeb handles SIGTERM/SIGINT: persist state, close listeners, exit 0
# within ~5s — give the supervisor margin beyond that in compose.
STOPSIGNAL SIGTERM
ENTRYPOINT ["/hivex"]
CMD ["--web-port", "7891", "--no-open"]
