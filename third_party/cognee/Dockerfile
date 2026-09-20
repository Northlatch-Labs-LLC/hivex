# Cognee — self-hosted AI memory engine for the Hivex Harness.
#
# Derived from the upstream image (Apache-2.0, topoteretes/cognee) with the
# gliner2 local entity extractor baked in, so the memory pipeline runs fully
# keyless: no cloud LLM, no external extraction service — inference-grade
# memory with zero cloud dependency, per the Northlatch deployment doctrine.
#
# Build:  docker build -t northlatch-cognee:local .
# Run:    see 9router/docker-compose.yml (service "cognee", host port 8090)

FROM cognee/cognee:main

# Bake the keyless local extractor exactly as cognee's [gliner] extra pins it
# (METADATA: gliner2[local]<3,>=2.0.0 + protobuf + sentencepiece), installed
# into the app's venv at /app/.venv — the app runs under that interpreter.
USER root
RUN /app/.venv/bin/python -m ensurepip --upgrade \
 && /app/.venv/bin/python -m pip install --no-cache-dir \
    "gliner2[local]>=2.0.0,<3" \
    "protobuf>=5.29.6" \
    "sentencepiece>=0.2.0"

# Fastembed downloads its embedding model on first use; keep the cache on the
# mounted /data volume so restarts never re-download. The app runs as the
# unprivileged `cognee` user, so the mount point must be owned by it — a
# fresh named volume inherits this ownership on first mount.
RUN mkdir -p /data/fastembed_cache && chown -R cognee:cognee /data
USER cognee

ENV FASTEMBED_CACHE_PATH=/data/fastembed_cache
