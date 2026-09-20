import { describe, expect, it } from "bun:test";

import { HiveAPIClient } from "../src/client.ts";

/**
 * Wire-shape checks for every resource against the REAL gateway routes
 * (9router/src/app/api/v1/…). Each test stubs fetch and asserts the exact
 * method, path, body encoding and response parsing.
 */

interface CapturedRequest {
  url: string;
  method: string;
  headers: Headers;
  body: string | FormData | null;
}

function captureClient(responder: (req: CapturedRequest) => Response | Promise<Response>) {
  const calls: CapturedRequest[] = [];
  const client = new HiveAPIClient({
    baseURL: "https://gw.example",
    apiKey: "sk-fixedmachine-abc123-a1b2c3d4",
    fetchImpl: (async (url: string | URL | Request, init?: RequestInit) => {
      const href = typeof url === "string" ? url : url instanceof URL ? url.href : url.url;
      const body =
        typeof init?.body === "string"
          ? init.body
          : init?.body instanceof FormData
            ? init.body
            : null;
      const req: CapturedRequest = {
        url: href,
        method: init?.method ?? "GET",
        headers: new Headers(init?.headers),
        body,
      };
      calls.push(req);
      return responder(req);
    }),
  });
  return { client, calls };
}

function okJson(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

describe("models.list — GET /v1/models", () => {
  it("hits the real route and parses the OpenAI list shape", async () => {
    const modelsBody = {
      object: "list",
      data: [
        { id: "openai/gpt-5", object: "model", owned_by: "openai", context_length: 400000, max_completion_tokens: 128000 },
        { id: "cc/claude-opus-4-7", object: "model", owned_by: "cc" },
        { id: "vip", object: "model", owned_by: "combo" },
      ],
    };
    const { client, calls } = captureClient(() => okJson(modelsBody));
    const models = await client.models.list();
    expect(models.object).toBe("list");
    expect(models.data).toHaveLength(3);
    expect(models.data[0]?.id).toBe("openai/gpt-5");
    const req = calls[0];
    expect(req?.url).toBe("https://gw.example/v1/models");
    expect(req?.method).toBe("GET");
    expect(req?.headers.get("authorization")).toBe("Bearer sk-fixedmachine-abc123-a1b2c3d4");
  });
});

describe("embeddings.create — POST /v1/embeddings", () => {
  it("sends the OpenAI shape and parses the list response", async () => {
    const embeddingsBody = {
      object: "list",
      model: "openai/text-embedding-3-small",
      data: [
        { object: "embedding", index: 0, embedding: [0.0123, -0.045, 0.678] },
        { object: "embedding", index: 1, embedding: [0.321, 0.456] },
      ],
      usage: { prompt_tokens: 5, total_tokens: 5 },
    };
    const { client, calls } = captureClient(() => okJson(embeddingsBody));
    const result = await client.embeddings.create({
      model: "openai/text-embedding-3-small",
      input: ["hello", "world"],
      dimensions: 512,
    });
    expect(result.data[1]?.embedding).toEqual([0.321, 0.456]);
    expect(result.usage.total_tokens).toBe(5);
    const req = calls[0];
    expect(req?.url).toBe("https://gw.example/v1/embeddings");
    expect(req?.method).toBe("POST");
    const sent = JSON.parse(String(req?.body));
    expect(sent.model).toBe("openai/text-embedding-3-small");
    expect(sent.input).toEqual(["hello", "world"]);
    expect(sent.dimensions).toBe(512);
  });
});

describe("images.generate — POST /v1/images/generations", () => {
  it("sends the JSON shape by default", async () => {
    const { client, calls } = captureClient(() =>
      okJson({ created: 1735000000, data: [{ url: "https://cdn.example/img.png" }] }),
    );
    const result = await client.images.generate({
      model: "gemini/gemini-3-pro-image-preview",
      prompt: "neon city",
      size: "1024x1024",
    });
    expect(result.data[0]?.url).toBe("https://cdn.example/img.png");
    const req = calls[0];
    expect(req?.url).toBe("https://gw.example/v1/images/generations");
    expect(JSON.parse(String(req?.body)).prompt).toBe("neon city");
  });

  it("uses the real ?response_format=binary query for raw bytes", async () => {
    const pngBytes = new Uint8Array([0x89, 0x50, 0x4e, 0x47]);
    const { client, calls } = captureClient(
      () => new Response(pngBytes, { status: 200, headers: { "Content-Type": "image/png" } }),
    );
    const bytes = await client.images.generate(
      { model: "gemini/gemini-3-pro-image-preview", prompt: "watercolor mountains" },
      { binary: true },
    );
    expect(new Uint8Array(bytes)).toEqual(pngBytes);
    expect(calls[0]?.url).toBe(
      "https://gw.example/v1/images/generations?response_format=binary",
    );
  });
});

describe("audio.speech — POST /v1/audio/speech", () => {
  it("returns raw bytes by default (gateway answers audio/mp3)", async () => {
    const mp3 = new Uint8Array([1, 2, 3, 4]);
    const { client, calls } = captureClient(
      () => new Response(mp3, { status: 200, headers: { "Content-Type": "audio/mp3" } }),
    );
    const bytes = await client.audio.speech({ model: "openai/tts-1", input: "Hello world" });
    expect(new Uint8Array(bytes)).toEqual(mp3);
    const req = calls[0];
    expect(req?.url).toBe("https://gw.example/v1/audio/speech");
    expect(JSON.parse(String(req?.body))).toEqual({ model: "openai/tts-1", input: "Hello world" });
  });

  it("uses the real ?response_format=json query and parses {audio, format}", async () => {
    const { client, calls } = captureClient(() =>
      okJson({ audio: "SUQzBAAAAAA=", format: "mp3" }),
    );
    const result = await client.audio.speech(
      { model: "el/eleven_multilingual_v2", input: "Xin chào" },
      { responseFormat: "json" },
    );
    expect(result.format).toBe("mp3");
    expect(result.audio.startsWith("SUQz")).toBe(true);
    expect(calls[0]?.url).toBe("https://gw.example/v1/audio/speech?response_format=json");
  });
});

describe("audio.transcriptions — POST /v1/audio/transcriptions (multipart)", () => {
  it("sends multipart/form-data with model + file and parses {text}", async () => {
    const { client, calls } = captureClient(() =>
      okJson({ text: "Xin chào, đây là bản ghi âm." }),
    );
    const audio = new Uint8Array([0, 1, 2, 3, 4, 5, 6, 7]);
    const result = await client.audio.transcriptions({
      model: "openai/whisper-1",
      file: audio,
      filename: "audio.mp3",
      language: "vi",
    });
    expect(result).toEqual({ text: "Xin chào, đây là bản ghi âm." });
    const req = calls[0];
    expect(req?.url).toBe("https://gw.example/v1/audio/transcriptions");
    expect(req?.method).toBe("POST");
    const form = req?.body as FormData;
    expect(form).toBeInstanceOf(FormData);
    expect(form.get("model")).toBe("openai/whisper-1");
    expect(form.get("language")).toBe("vi");
    const file = form.get("file") as File;
    expect(file.name).toBe("audio.mp3");
    expect(new Uint8Array(await file.arrayBuffer())).toEqual(audio);
    // Multipart bodies must not carry a manual Content-Type — fetch adds the
    // boundary itself; a hand-set header would break multipart parsing.
    expect(req?.headers.get("content-type")).toBeNull();
  });

  it("returns raw text for text/srt/vtt response formats", async () => {
    const { client, calls } = captureClient(
      () => new Response("1\n00:00:01,000 --> 00:00:02,000\nHello", { status: 200 }),
    );
    const srt = await client.audio.transcriptions({
      model: "groq/whisper-large-v3",
      file: new Blob([new Uint8Array([9, 9])]),
      response_format: "srt",
    });
    expect(srt).toContain("00:00:01,000");
    const form = calls[0]?.body as FormData;
    expect(form.get("response_format")).toBe("srt");
  });
});

describe("search.create — POST /v1/search", () => {
  it("sends the unified search body and parses the unified response", async () => {
    const searchBody = {
      provider: "tavily",
      query: "HiveAPI Gateway open source",
      results: [
        {
          title: "HiveAPI Gateway",
          url: "https://github.com/Northlatch-Labs-LLC/hiveapi",
          display_url: "github.com/Northlatch-Labs-LLC/hiveapi",
          snippet: "Open-source LLM gateway",
          position: 1,
          score: 0.92,
          published_at: null,
          favicon_url: null,
          content: null,
          metadata: { author: null, language: null, source_type: null, image_url: null },
          citation: { provider: "tavily", retrieved_at: "2026-09-19T00:00:00Z", rank: 1 },
        },
      ],
      answer: null,
      usage: { queries_used: 1, search_cost_usd: 0.008 },
      metrics: { response_time_ms: 850, upstream_latency_ms: 700, total_results_available: 12 },
      errors: [],
    };
    const { client, calls } = captureClient(() => okJson(searchBody));
    const result = await client.search.create({
      model: "tavily",
      query: "HiveAPI Gateway open source",
      max_results: 5,
    });
    expect(result.provider).toBe("tavily");
    expect(result.results[0]?.url).toBe("https://github.com/Northlatch-Labs-LLC/hiveapi");
    expect(result.usage?.queries_used).toBe(1);
    const req = calls[0];
    expect(req?.url).toBe("https://gw.example/v1/search");
    const sent = JSON.parse(String(req?.body));
    expect(sent.model).toBe("tavily");
    expect(sent.query).toBe("HiveAPI Gateway open source");
    expect(sent.max_results).toBe(5);
  });
});

describe("request() passthrough", () => {
  it("exposes GET/POST for arbitrary routes under /v1 (e.g. the real /v1/models/info)", async () => {
    const { client, calls } = captureClient(() =>
      okJson({ id: "openai/gpt-4o", kind: "llm", endpoint: "/v1/chat/completions" }),
    );
    const res = await client.request("models/info?id=openai/gpt-4o");
    expect(res.status).toBe(200);
    const req = calls[0];
    expect(req?.url).toBe("https://gw.example/v1/models/info?id=openai/gpt-4o");
    expect(req?.method).toBe("GET");
    // Portal routes (/api/portal/*) live OUTSIDE /v1 — the MCP package builds
    // those URLs against the raw base, never through this method.
    const res2 = await client.request("chat/completions", {
      method: "POST",
      body: { model: "x", messages: [] },
    });
    expect(res2.status).toBe(200);
    expect(calls[1]?.url).toBe("https://gw.example/v1/chat/completions");
  });
});
