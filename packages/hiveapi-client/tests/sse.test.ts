import { describe, expect, it } from "bun:test";

import { iterateSSE, SSE_DONE } from "../src/sse.ts";
import {
  chatTextCrlfFrames,
  chatTextFrames,
  chatToolCallFrames,
  framesToBytes,
  framesToChunkedStream,
  framesToStream,
  parserEdgeCaseFrames,
} from "./fixtures.ts";

async function collect(stream: ReadableStream<Uint8Array>) {
  const events = [];
  for await (const event of iterateSSE(stream)) events.push(event);
  return events;
}

describe("iterateSSE — streaming parse", () => {
  it("parses gateway text frames in order", async () => {
    const events = await collect(framesToStream(chatTextFrames));
    // All frames except the sentinel are chat.completion.chunk JSON.
    expect(events).toHaveLength(chatTextFrames.length);
    expect(events[0]?.data).toContain('"object":"chat.completion.chunk"');
    const roleFrame = JSON.parse(events[0]?.data ?? "{}");
    expect(roleFrame.choices[0].delta.role).toBe("assistant");
    const contentFrame = JSON.parse(events[1]?.data ?? "{}");
    expect(contentFrame.choices[0].delta.content).toBe("Hello");
    expect(events.at(-1)?.data).toBe(SSE_DONE);
  });

  it("parses the [DONE] sentinel as its own event", async () => {
    const events = await collect(framesToStream(chatToolCallFrames));
    expect(events.at(-1)).toEqual({ data: SSE_DONE, event: undefined });
    // Tool-call fragments: head + 3 argument pieces + finish + usage + DONE.
    const toolFrames = events
      .filter((e) => e.data !== SSE_DONE)
      .map((e) => JSON.parse(e.data))
      .filter((f) => f?.choices?.[0]?.delta?.tool_calls);
    expect(toolFrames).toHaveLength(4);
    expect(toolFrames[0].choices[0].delta.tool_calls[0].function.name).toBe("get_weather");
    expect(toolFrames[3].choices[0].delta.tool_calls[0].function.arguments).toBe(
      '"unit":"celsius"}',
    );
  });

  it("reassembles frames split across tiny chunks", async () => {
    const whole = await collect(framesToStream(chatTextFrames));
    const chunked = await collect(framesToChunkedStream(chatTextFrames, 7));
    expect(chunked).toEqual(whole);
  });

  it("handles CRLF line endings", async () => {
    const events = await collect(framesToStream(chatTextCrlfFrames));
    expect(events).toHaveLength(chatTextFrames.length);
    expect(events[1] && JSON.parse(events[1].data).choices[0].delta.content).toBe("Hello");
    expect(events.at(-1)?.data).toBe(SSE_DONE);
  });

  it("ignores comments, captures event fields, joins multi-line data", async () => {
    const events = await collect(framesToStream(parserEdgeCaseFrames));
    expect(events).toHaveLength(3);
    expect(events[0]?.event).toBe("message");
    expect(JSON.parse(events[0]?.data ?? "{}")).toEqual({ hello: "line one" });
    expect(events[1]?.data).toBe("first line\nsecond line");
    expect(events[2]?.data).toBe(SSE_DONE);
  });

  it("flushes a trailing unterminated frame", async () => {
    const bytes = framesToBytes(chatTextFrames.slice(0, 2)); // no [DONE]
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(bytes);
        controller.close();
      },
    });
    const events = await collect(stream);
    expect(events).toHaveLength(2);
    expect(JSON.parse(events[1]?.data ?? "{}").choices[0].delta.content).toBe("Hello");
  });
});
