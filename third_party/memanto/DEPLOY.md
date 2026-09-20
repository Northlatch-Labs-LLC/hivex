# Memanto — Self-Hosted Deployment (Northlatch)

Memanto is the Hive product line's agent-memory service: a Python "memory
agent" that stores, consolidates, reconciles, and briefs memories for the
Hivex Harness's bots. Vendored at `third_party/memanto` (upstream:
moorcheh-ai/memanto, MIT © 2026 EdgeAI Innovations Inc. — see its LICENSE).

## Run it

Memanto ships its own `Dockerfile` + `docker-compose.yml`. From
`third_party/memanto/`:

```bash
cp .env.example .env   # then fill MOORCHEH_API_KEY
docker compose up -d
```

**Port contract (Northlatch standard):** Memanto's container listens on
`8000` internally. On our infra the host port `8000` is reserved for the
hosting provider panel, so the Northlatch deployment maps it to **8090**:

```yaml
services:
  memanto:
    build: .
    image: memanto:latest
    ports:
      - "8090:8000"        # Northlatch remap (host 8000 = hosting panel)
    env_file:
      - .env
    restart: unless-stopped
    volumes:
      - memanto-state:/data   # ADD: persist the memory estate across rebuilds
    healthcheck:
      test: ["CMD", "python", "-c",
             "import urllib.request; urllib.request.urlopen('http://localhost:8000/ready')"]
      interval: 30s
      timeout: 10s
      retries: 3
volumes:
  memanto-state:
```

## Environment

| Var | Purpose |
| --- | --- |
| `MOORCHEH_API_KEY` | Memanto's own API key (required by the service). |
| `MEMANTO_PORT` | Internal listen port (default `8000`). |

## Health checks

- `GET /ready` — ready to serve (compose healthcheck target)
- `GET /live` — liveness
- `GET /health` — service health
- `GET /status` — operational status

## Verify the deployment

```bash
curl -fsS http://localhost:8090/ready
curl -fsS -H "Authorization: Bearer $MOORCHEH_API_KEY" http://localhost:8090/status
```

## The harness contract

The Hivex Harness targets this instance with:

```
HIVEX_MEMANTO_URL=http://localhost:8090
HIVEX_MEMANTO_API_KEY=<MOORCHEH_API_KEY>
```

and selects it with `HIVEX_MEMORY_BACKEND=memanto` (see INTEGRATION.md).
