/**
 * End-to-end smoke test: a Bun.serve mock of the gateway's real routes,
 * hit with the default global fetch (no fetchImpl stub) — a complete
 * streaming chat call plus a models call over a real socket.
 */

import { afterAll, beforeAll, describe, expect, it } from "bun:test";

import { HiveAPIClient } from "../src/client.ts";
import { chatTextFrames, chatToolCallFrames, expectedToolCallArguments } from "./fixtures.ts";

let server: ReturnType<typeof Bun.serve> | null = null;
let serverURL = "";
const seenAuth: string[] = [];

beforeAll(() => {
  server = Bun.serve({
    port: 0,
    idleTimeout: 10,
    async fetch(request) {
      const url = new URL(request.url);
      if (request.headers.get("authorization")) {
        seenAuth.push(request.headers.get("authorization") as string);
      }

      if (url.pathname === "/v1/models" && request.method === "GET") {
        return Response.json({
          object: "list",
          data: [
            { id: "openai/gpt-5", object: "model", owned_by: "openai" },
            { id: "tavily/search", object: "model", kind: "webSearch", owned_by: "tavily" },
          ],
        });
      }

      if (url.pathname === "/v1/chat/completions" && request.method === "POST") {
        const body = (await request.json()) as { stream?: boolean; model?: string };
        if (body.stream !== true) {
          return Response.json({
            id: "chatcmpl-smoke-plain",
            object: "chat.completion",
            created: 1758326400,
            model: body.model ?? "openai/gpt-5",
            choices: [
              {
                index: 0,
                message: { role: "assistant", content: "Hello from the mock gateway!" },
                finish_reason: "stop",
              },
            ],
            usage: { prompt_tokens: 8, completion_tokens: 2, total_tokens: 10 },
          });
        }
        const frames =
          body.model === "openai/toolbox"
            ? chatToolCallFrames.join("")
            : chatTextFrames.join("");
        const bytes = new TextEncoder().encode(frames);
        return new Response(new ReadableStream<Uint8Array>({
          start(controller) {
            controller.enqueue(bytes);
            controller.close();
          },
        }), {
          status: 200,
          headers: {
            "Content-Type": "text/event-stream",
            "Cache-Control": "no-cache",
          },
        });
      }

      return Response.json({ error: { message: `No route: ${url.pathname}` } }, { status: 404 });
    },
  });
  serverURL = server.url.toString().replace(/\/$/, "");
});

afterAll(() => {
  server?.stop(true);
  server = null;
});

describe("smoke — Bun.serve gateway mock", () => {
  it("completes a streaming chat call end-to-end", async () => {
    const client = new HiveAPIClient({ baseURL: serverURL, apiKey: "sk-smokemachine-k3y777-cafe1234" });
    const stream = await client.chat.completions.create({
      model: "openai/gpt-5",
      messages: [{ role: "user", content: "Say hello" }],
      stream: true,
      stream_options: { include_usage: true },
    });

    let text = "";
    let chunkCount = 0;
    for await (const chunk of stream) {
      chunkCount += 1;
      text += chunk.choices[0]?.delta?.content ?? "";
    }
    expect(text).toBe("Hello, world!");
    expect(chunkCount).toBe(6); // [DONE] terminates the iteration, not a chunk
  });

  it("folds tool-call fragments over a real socket", async () => {
    const client = new HiveAPIClient({ baseURL: serverURL, apiKey: "sk-smokemachine-k3y777-cafe1234" });
    const stream = await client.chat.completions.create({
      model: "openai/toolbox",
      messages: [{ role: "user", content: "Weather in Hanoi?" }],
      stream: true,
    });
    const final = await stream.finalMessage();
    expect(final.tool_calls).toHaveLength(1);
    expect(final.tool_calls[0]?.function.arguments).toBe(expectedToolCallArguments);
    expect(final.finish_reason).toBe("tool_calls");
  });

  it("completes a non-streaming chat call and a models call", async () => {
    const client = new HiveAPIClient({ baseURL: serverURL, apiKey: "sk-smokemachine-k3y777-cafe1234" });
    const completion = await client.chat.completions.create({
      model: "openai/gpt-5",
      messages: [{ role: "user", content: "Say hello" }],
    });
    expect(completion.choices[0]?.message.content).toBe("Hello from the mock gateway!");
    expect(completion.usage?.total_tokens).toBe(10);

    const models = await client.models.list();
    expect(models.object).toBe("list");
    expect(models.data.map((m) => m.id)).toContain("tavily/search");
  });

  it("sends Bearer auth on every request", () => {
    expect(seenAuth.length).toBeGreaterThanOrEqual(3);
    for (const auth of seenAuth) {
      expect(auth).toBe("Bearer sk-smokemachine-k3y777-cafe1234");
    }
  });
});
