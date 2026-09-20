import { describe, expect, it } from "bun:test";

import { HiveAPIClient } from "../src/client.ts";
import { HiveAPIError } from "../src/errors.ts";
import type { ChatCompletion } from "../src/types.ts";
import {
  chatCompletionBody,
  chatErrorFrames,
  chatTextFrames,
  chatToolCallFrames,
  expectedToolCallArguments,
  framesToStream,
} from "./fixtures.ts";

function sseResponse(frames: string[]): Response {
  return new Response(framesToStream(frames), {
    status: 200,
    headers: { "Content-Type": "text/event-stream; charset=utf-8" },
  });
}

function clientWith(handler: (url: string, init?: RequestInit) => Response | Promise<Response>) {
  const calls: { url: string; init?: RequestInit }[] = [];
  const fetchImpl = (async (url: string | URL | Request, init?: RequestInit) => {
    const href = typeof url === "string" ? url : url instanceof URL ? url.href : url.url;
    calls.push({ url: href, init });
    return handler(href, init);
  });
  const client = new HiveAPIClient({
    baseURL: "https://gw.example",
    apiKey: "sk-fixedmachine-abc123-a1b2c3d4",
    fetchImpl,
  });
  return { client, calls };
}

describe("chat.completions.create — streaming", () => {
  it("iterates gateway SSE chunks and stops at [DONE]", async () => {
    const { client } = clientWith(() => sseResponse(chatTextFrames));
    const stream = await client.chat.completions.create({
      model: "openai/gpt-5",
      messages: [{ role: "user", content: "Hi" }],
      stream: true,
    });
    const chunks = await stream.toArray();
    // 6 wire chunks: role, 3 content, stop, usage — [DONE] is consumed, not yielded.
    expect(chunks).toHaveLength(6);
    expect(chunks[0]?.choices[0]?.delta.role).toBe("assistant");
    expect(chunks.at(-1)?.usage?.total_tokens).toBe(60);
  });

  it("folds a plain-text stream with finalMessage()", async () => {
    const { client } = clientWith(() => sseResponse(chatTextFrames));
    const stream = await client.chat.completions.create({
      model: "openai/gpt-5",
      messages: [{ role: "user", content: "Hi" }],
      stream: true,
    });
    const final = await stream.finalMessage();
    expect(final.content).toBe("Hello, world!");
    expect(final.finish_reason).toBe("stop");
    expect(final.tool_calls).toEqual([]);
    expect(final.usage?.prompt_tokens).toBe(42);
    expect(final.usage?.total_tokens).toBe(60);
    expect(final.model).toBe("openai/gpt-5");
    expect(final.id).toBe("chatcmpl-hiveapi-fix01");
  });

  it("accumulates tool-call fragments into one tool call", async () => {
    const { client } = clientWith(() => sseResponse(chatToolCallFrames));
    const stream = await client.chat.completions.create({
      model: "openai/gpt-5",
      messages: [{ role: "user", content: "Weather in Hanoi?" }],
      tools: [
        {
          type: "function",
          function: {
            name: "get_weather",
            description: "Get the weather for a city",
            parameters: {
              type: "object",
              properties: { city: { type: "string" }, unit: { type: "string" } },
              required: ["city"],
            },
          },
        },
      ],
      stream: true,
    });
    const final = await stream.finalMessage();
    expect(final.content).toBe("");
    expect(final.reasoning_content).toBe("Checking the weather service...");
    expect(final.tool_calls).toHaveLength(1);
    const call = final.tool_calls[0];
    expect(call?.id).toBe("toolu_hive01ABCdefGHIjklMNOpq");
    expect(call?.function.name).toBe("get_weather");
    expect(call?.function.arguments).toBe(expectedToolCallArguments);
    expect(JSON.parse(call?.function.arguments ?? "{}")).toEqual({
      city: "Hanoi",
      unit: "celsius",
    });
    expect(final.finish_reason).toBe("tool_calls");
  });

  it("surfaces mid-stream error frames as classified HiveAPIError", async () => {
    const { client } = clientWith(() => sseResponse(chatErrorFrames));
    const stream = await client.chat.completions.create({
      model: "openai/gpt-5",
      messages: [{ role: "user", content: "Hi" }],
      stream: true,
    });
    const chunks = [];
    try {
      for await (const chunk of stream) chunks.push(chunk);
      expect.unreachable();
    } catch (error) {
      expect(error).toBeInstanceOf(HiveAPIError);
      const hiveError = error as HiveAPIError;
      expect(hiveError.kind).toBe("rate_limited");
      expect(hiveError.type).toBe("rate_limit_error");
      expect(hiveError.message).toContain("Rate limit exceeded");
    }
    expect(chunks).toHaveLength(2); // role + one content delta before the error frame
  });

  it("falls back to synthetic chunks when the server answers JSON to stream:true", async () => {
    const { client } = clientWith(
      () =>
        new Response(JSON.stringify(chatCompletionBody), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    );
    const stream = await client.chat.completions.create({
      model: "openai/gpt-5",
      messages: [{ role: "user", content: "Hi" }],
      stream: true,
    });
    const final = await stream.finalMessage();
    expect(final.content).toBe("Hello from the HiveAPI Gateway!");
    expect(final.finish_reason).toBe("stop");
    expect(final.usage?.total_tokens).toBe(10);
    expect(final.tool_calls).toEqual([]);
  });
});

describe("chat.completions.create — non-streaming", () => {
  it("returns a parsed chat.completion", async () => {
    const { client, calls } = clientWith(
      () =>
        new Response(JSON.stringify(chatCompletionBody), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    );
    const completion = await client.chat.completions.create({
      model: "openai/gpt-5",
      messages: [{ role: "user", content: "Hi" }],
    });
    expect(completion.object).toBe("chat.completion");
    expect(completion.choices[0]?.message.content).toBe("Hello from the HiveAPI Gateway!");
    expect(completion.usage?.total_tokens).toBe(10);
    // Wire check: POST to the real route, Bearer auth, JSON content type.
    expect(calls[0]?.url).toBe("https://gw.example/v1/chat/completions");
    expect((calls[0]?.init as RequestInit).method).toBe("POST");
    const headers = new Headers((calls[0]?.init as RequestInit).headers);
    expect(headers.get("authorization")).toBe("Bearer sk-fixedmachine-abc123-a1b2c3d4");
    expect(headers.get("content-type")).toBe("application/json");
  });

  it("folds the stream when the server streams despite stream:false", async () => {
    const { client } = clientWith(() => sseResponse(chatTextFrames));
    const completion: ChatCompletion = await client.chat.completions.create({
      model: "openai/gpt-5",
      messages: [{ role: "user", content: "Hi" }],
    });
    expect(completion.object).toBe("chat.completion");
    expect(completion.choices[0]?.message.content).toBe("Hello, world!");
    expect(completion.choices[0]?.finish_reason).toBe("stop");
    expect(completion.usage?.total_tokens).toBe(60);
  });
});
