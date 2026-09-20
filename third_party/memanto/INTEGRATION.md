# Memanto — Hivex Harness Integration Contract

How the Hivex Harness (`hivebot`, Go) integrates the self-hosted Memanto
memory service as the `memanto` organizational-memory backend
(`HIVEX_MEMORY_BACKEND=memanto`), replacing every retired "nex" backend path.

## Doctrine (from docs/specs/core-loop.md)

Deterministic hooks decide **what** is remembered and **when** — never the
LLM's whim. The only write path is the post-task distillation hook
(`task_distill.go`), gated on the machine-verification result; the only read
path is context assembly. Memanto is the store/operations substrate: it
consolidates, reconciles conflicts, and applies forgetting policies on its
own schedule — but the harness only ever writes verified outcomes and
reflexion-style failure lessons.

## Environment contract

| Var | Meaning |
| --- | --- |
| `HIVEX_MEMANTO_URL` | Base URL of the self-hosted instance (e.g. `http://localhost:8090`). No trailing `/v1`; Memanto's routes are at the root. |
| `HIVEX_MEMANTO_API_KEY` | Bearer token (the instance's `MOORCHEH_API_KEY`). |

Auth on every call: `Authorization: Bearer <HIVEX_MEMANTO_API_KEY>`.

## Wire surface used by the harness

Memanto scopes its memory operations per agent. Map each bot slug 1:1 to a
Memanto `agent_id` (the broker's bot slugs are already filesystem-safe).

| Harness operation | Memanto endpoint | Notes |
| --- | --- | --- |
| Ensure an agent exists (lazy, on first turn) | `GET /agents` then `POST /agents` | Register bot slugs once; reuse thereafter. |
| Write verified learnings (distill hook) | `POST /{agent_id}/batch-remember` | One call per distilled task outcome: facts, insights, failure lessons — each with metadata (`task_id`, `verified: true`, `kind`). |
| Retrieve for context assembly | `POST /{agent_id}/answer` | Query = task title + details + trigger; Memanto returns the minimal relevant slice. Use for the "context I used" packet. |
| List/inspect (Librarian surfaces) | `GET /{agent_id}/memories` family | Read-only browsing for the wiki/notebook UIs. |
| Expire a stale fact | `POST /{agent_id}/memories/{memory_id}/expire` | Used by the update-first discipline when a learning is superseded. |
| Conflict surfacing | `GET /{agent_id}/conflicts` | Surfaced in the Librarian's review queue; resolution stays a human/curator action via `POST /{agent_id}/conflicts/resolve`. |
| Forgetting policy | `GET /{agent_id}/policy/presets`, `POST /{agent_id}/policy/apply` | Apply a decay preset per agent; defaults live in Memanto, not the harness. |
| Daily consolidation | Memanto's own scheduler | No harness action; `/status` is checked by the doctor command. |

### Write example (distill hook → batch-remember)

```bash
curl -fsS -X POST "$HIVEX_MEMANTO_URL/hive-eng/agent-001/batch-remember" \
  -H "Authorization: Bearer $HIVEX_MEMANTO_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "memories": [
      {
        "content": "Verified: the broker must write broker.json atomically (tmp+rename) — direct writes corrupted state twice (task #1042).",
        "metadata": {"task_id": "1042", "kind": "lesson", "verified": true, "confidence": 0.9}
      }
    ]
  }'
```

(The exact body field names must be confirmed against the vendored
`openapi.json` (`third_party/memanto/sdks/typescript/openapi.json`) at
implementation time — that file is the machine-readable contract.)

### Read example (context assembly → answer)

```bash
curl -fsS -X POST "$HIVEX_MEMANTO_URL/agent-001/answer" \
  -H "Authorization: Bearer $HIVEX_MEMANTO_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"question": "What do we know about atomic broker-state writes?", "top_k": 8}'
```

## Proposed Go backend interface (internal team memory seam)

```go
// MemoryBackend is the seam HIVEX_MEMORY_BACKEND selects. The memanto
// implementation speaks HTTP to HIVEX_MEMANTO_URL; gbrain/markdown/none
// keep their existing implementations.
type MemoryBackend interface {
    // StoreVerified writes one or more verification-gated memories for a bot.
    StoreVerified(ctx context.Context, bot string, mems []MemoryRecord) error
    // Retrieve returns the minimal relevant slice for a context query.
    Retrieve(ctx context.Context, bot, query string, topK int) ([]MemoryRecord, error)
    // List/Expire support the Librarian's curation surfaces.
    List(ctx context.Context, bot string) ([]MemoryRecord, error)
    Expire(ctx context.Context, bot, memoryID, reason string) error
}

type MemoryRecord struct {
    ID       string
    Content  string
    Metadata map[string]any // task_id, kind, verified, confidence, source
}
```

Wiring: `internal/config.NormalizeMemoryBackend` gains
`MemoryBackendMemanto = "memanto"`; the resolver health-checks the instance
(`GET /ready`) so `hivebot doctor` reports it like gbrain readiness. The
distill hook's `AppendVerified` trust path routes `StoreVerified`; the
context assembler's retrieval seam routes `Retrieve` with the hybrid
fallback to the existing wiki BM25 when Memanto is unreachable (never brick
a turn on the memory service).

## Two-tier memory semantics

- **Private per-agent KG → notebooks:** each bot's Memanto agent holds its
  own memories (pre-task research, post-task learnings). Notebooks remain
  generated views.
- **Team KG → wiki:** the Librarian agent is a regular Memanto agent
  (`agent_id: librarian`); verified facts promoted to the wiki are ALSO
  batch-remembered there so team-wide retrieval answers from one estate.
  The private/team boundary stays an access decision in the harness
  (permissioned access per core-loop.md), enforced by which agent_id a
  query targets — never by Memanto internals.

## Gaps to confirm at implementation time

1. Exact request schemas from `openapi.json` (field names above are
   representative; the SDK (`sdks/typescript/src`) is the typed reference.
2. Rate/retry posture: Memanto sits on the broker's own loopback network —
   use short timeouts (2s) + fail-open on the read path, fail-loud on the
   write path (a distill that cannot persist must retry, not drop).
3. The vendored copy pins upstream `main`; pin to a released tag before
   production (quarantine rule for third-party deps).
