# @hiveapi/client

Typed, zero-runtime-dependency TypeScript client for the **HiveAPI Gateway**.
It speaks the gateway's OpenAI-compatible surface over global `fetch` and Web
Streams — no SDK, no polyfills, works in Bun, Node 18+, browsers and edge
runtimes.

## Environment contract

| Variable | Required | Meaning |
|---|---|---|
| `HIVEAPI_BASE_URL` | yes | Gateway base URL. `/v1` is auto-appended when missing; an existing query string is preserved on every request. |
| `HIVEAPI_API_KEY` | no | Gateway API key (Bearer). Omit for a local gateway with `requireApiKey=false`. |

```bash
export HIVEAPI_BASE_URL="https://gw.example.com"
export HIVEAPI_API_KEY="sk-a1b2c3d4e5f6a7b8-abc123-a1b2c3d4"
```

## Quickstart

```bash
bun add @hiveapi/client
```

```ts
import { HiveAPIClient } from "@hiveapi/client";

const client = new HiveAPIClient({
  baseURL: process.env.HIVEAPI_BASE_URL!,
  apiKey: process.env.HIVEAPI_API_KEY,
});

// Non-streaming chat
const completion = await client.chat.completions.create({
  model: "openai/gpt-5",
  messages: [{ role: "user", content: "Hi!" }],
});
console.log(completion.choices[0].message.content);
```

### Streaming (SSE)

`stream: true` returns a `ChatStream` — an async iterator over the gateway's
`chat.completion.chunk` frames. The `data: [DONE]` sentinel ends iteration,
mid-stream error frames become classified errors, and `finalMessage()` folds
content, **tool-call fragments** and usage into one assistant message.

```ts
const stream = await client.chat.completions.create({
  model: "openai/gpt-5",
  messages: [{ role: "user", content: "Weather in Hanoi?" }],
  tools: [{
    type: "function",
    function: {
      name: "get_weather",
      parameters: { type: "object", properties: { city: { type: "string" } } },
    },
  }],
  stream: true,
  stream_options: { include_usage: true }, // ask for the terminal usage frame
});

for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0]?.delta?.content ?? "");
}

// …or fold the whole turn:
const final = await stream.finalMessage();
console.log(final.tool_calls[0]?.function.arguments); // {"city":"Hanoi", ...}
console.log(final.usage?.total_tokens);
```

If the gateway answers JSON to a `stream:true` request (non-stream fallback),
the stream yields synthetic chunks instead, so consumers keep one code path.

## API surface (mirrors the gateway's real routes)

| Method | Route | Notes |
|---|---|---|
| `chat.completions.create(params)` | `POST /v1/chat/completions` | `stream:true` → `ChatStream`; otherwise `ChatCompletion`. Tools, `stream_options.include_usage`, and all OpenAI params pass through. |
| `models.list()` | `GET /v1/models` | OpenAI model list (`{object:"list", data:[…]}`). |
| `embeddings.create(params)` | `POST /v1/embeddings` | `{model, input, encoding_format?, dimensions?}`. |
| `images.generate(params, {binary?})` | `POST /v1/images/generations` | JSON (`url`/`b64_json`) by default; `{binary:true}` adds the gateway's `?response_format=binary` query and returns raw bytes. |
| `audio.speech(params, {responseFormat?})` | `POST /v1/audio/speech` | Raw audio bytes (`ArrayBuffer`) by default; `{responseFormat:"json"}` → `{audio, format}`. |
| `audio.transcriptions(params)` | `POST /v1/audio/transcriptions` | Multipart (`model`, `file`, `language`, `prompt`, `temperature`, `response_format`). JSON formats → `{text}`; `text`/`srt`/`vtt` → string. |
| `search.create(params)` | `POST /v1/search` | Unified web search: `{model, query, max_results?, search_type?, country?, language?, time_range?, domain_filter?, provider_options?}`. |
| `request(route, init)` | any `/v1` route | Escape hatch (e.g. `models/info?id=…`). Portal/admin routes are **not** under `/v1` — build those against the raw base URL. |

## Base URL handling

```ts
new HiveAPIClient({ baseURL: "https://gw.example" }).baseURL;
// → "https://gw.example/v1"
new HiveAPIClient({ baseURL: "https://gw.example/v1" }).baseURL;
// → "https://gw.example/v1"          (no double append)
new HiveAPIClient({ baseURL: "https://gw.example?token=abc" }).baseURL;
// → "https://gw.example/v1?token=abc" (query string round-trips on every request)
```

Route-level query params (e.g. `audio/speech?response_format=json`) are merged
after base query params.

## Error classification

Failures throw `HiveAPIError` with a stable `kind`, parsed from the status code
and the gateway's `{error: {message, type, code}}` body:

| Status | `kind` |
|---|---|
| fetch/network failure | `unreachable` |
| 401, 402, 403 | `unauthorized` |
| 429, 503* | `rate_limited` |
| other 5xx | `server_error` |
| other 4xx | `invalid_request` |

\* The gateway answers **503 with a `Retry-After` header** when all upstream
accounts are exhausted (`unavailableResponse` in open-sse) — the client reads
`Retry-After` into `error.retryAfterSeconds`.

```ts
import { HiveAPIError } from "@hiveapi/client";

try {
  await client.models.list();
} catch (error) {
  if (error instanceof HiveAPIError && error.kind === "rate_limited") {
    console.log(`Retry in ${error.retryAfterSeconds}s`);
  }
}
```

Aborted requests (`AbortSignal`) rethrow the original `AbortError` untouched.

## Development

```bash
bun install   # from the repo root (bun workspaces)
cd packages/hiveapi-client
bun test      # unit tests + fixtures + Bun.serve smoke test
bun run typecheck  # tsc --noEmit
```

The test fixtures mirror the gateway's real SSE wire format
(`data: {chunk}\n\n` frames, tool-call fragment emission, the terminal usage
frame, and the `data: [DONE]\n\n` sentinel) as emitted by
`9router/open-sse`.

## License

MIT — see [LICENSE](./LICENSE). Copyright (c) 2026 Northlatch Labs LLC.
