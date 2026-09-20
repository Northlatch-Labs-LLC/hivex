# Cognee — Hivex Harness memory engine (self-hosted)

Cognee (Apache-2.0, https://github.com/topoteretes/cognee) is the deployed
local memory backend for the Hivex Harness — the replacement for the
Memanto backend. It runs **fully local and keyless**: local GLiNER entity
extraction + local fastembed embeddings, no cloud LLM, no external service.

## Deployment

The production service runs as part of the 9router compose stack:

```sh
cd 9router
docker compose up -d cognee   # builds ../third_party/cognee (this directory)
```

- Host port: **8090** (8000 on the host is the hosting panel).
- Health: `GET http://localhost:8090/health` → `{"status":"ready",...}`
  (compose healthcheck uses the same probe; start_period 90s).
- Persistence: named volume `cognee-data` at `/data` (fastembed model cache
  at `/data/fastembed_cache`); the image chowns `/data` to the unprivileged
  `cognee` user so a fresh volume inherits the right ownership.

## Image (Dockerfile)

Derived from `cognee/cognee:main` with, baked in:

- `gliner2[local]>=2.0.0,<3` + `protobuf` + `sentencepiece` — exactly what
  cognee's own `[gliner]` extra pins (its METADATA), installed into the
  app's pip-less uv venv via `ensurepip`. Without it, keyless cognify fails
  with `KeylessExtractorNotInstalledError`.
- `/data` owned by `cognee:cognee` (volume mount point).
- `FASTEMBED_CACHE_PATH=/data/fastembed_cache`.

## Harness wiring

```sh
export HIVEX_MEMORY_BACKEND=cognee
export HIVEX_COGNEE_URL=http://localhost:8090
# HIVEX_COGNEE_API_KEY is optional (only for multi-tenant deployments with
# ENABLE_BACKEND_ACCESS_CONTROL=true; this compose runs keyless).
```

The backend (internal/team/cognee_backend.go) implements the harness
`memoryBackend` interface against the empirically-verified v1.6 API:

- Read: `POST /api/v1/search` `{query, searchType: "CHUNKS", onlyContext: true, topK}` —
  pure retrieval, no server-side LLM; returns a bare JSON array of chunk
  strings (tolerant of object/envelope variants).
- Write: `POST /api/v1/add` (multipart `raw_data` + `datasetName`) followed by
  `POST /api/v1/cognify` `{datasets:[...]}` — cognify is REQUIRED: add alone
  does not make content retrievable.
- Team dataset: `hivex-team-estate` (constant in the backend).

Failure posture: reads fail open (empty), writes fail loud — same doctrine
as the memanto backend it replaces.

## Verification

- Unit tests: `go test ./internal/team/ -run TestCognee`
- LIVE round-trip (against a running instance):
  `HIVEX_TEST_COGNEE_URL=http://localhost:8090 go test ./internal/team/ -run TestCogneeLiveClientRoundTrip -v`
