/**
 * Minimal SSE reader for the gateway's client-facing wire format.
 *
 * The gateway streams OpenAI chunks as `data: {json}\n\n` frames terminated
 * by the `data: [DONE]\n\n` sentinel (9router/open-sse/utils/sseConstants.js,
 * streamHelpers.js — formatSSE). This parser yields the decoded payload of
 * each frame and treats the [DONE] payload like any other — the consumer
 * decides what to do with it.
 *
 * Handles CRLF line endings, multi-line `data:` fields, `event:` fields,
 * comment/heartbeat lines, and frames split across chunk boundaries.
 */

export interface SSEEvent {
  /** The joined payload of all `data:` lines in the frame. */
  data: string;
  /** The `event:` field value, when present. */
  event?: string;
}

/** The gateway's stream terminator payload. */
export const SSE_DONE = "[DONE]";

function parseEventBlock(block: string): SSEEvent | null {
  let event: string | undefined;
  const dataLines: string[] = [];

  for (const rawLine of block.split("\n")) {
    if (rawLine === "" || rawLine.startsWith(":")) continue; // blank / comment
    if (rawLine.startsWith("data:")) {
      const value = rawLine.slice(5);
      dataLines.push(value.startsWith(" ") ? value.slice(1) : value);
    } else if (rawLine.startsWith("event:")) {
      const value = rawLine.slice(6);
      event = (value.startsWith(" ") ? value.slice(1) : value) || undefined;
    }
    // Other fields (id:, retry:) are ignored — the gateway never sends them.
  }
  if (dataLines.length === 0) return null;
  return { data: dataLines.join("\n"), event };
}

function normalizeNewlines(text: string): string {
  return text.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
}

/**
 * Iterate the SSE frames of a response body. Yields once per complete frame;
 * an unterminated trailing frame (connection cut before the [DONE] sentinel)
 * is flushed as a final event if it has data lines.
 */
export async function* iterateSSE(
  body: ReadableStream<Uint8Array>,
): AsyncGenerator<SSEEvent> {
  const decoder = new TextDecoder();
  const reader = body.getReader();
  let buffer = "";

  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });

      // Hold back a single trailing "\r": it may be the first half of a
      // "\r\n" pair split across chunks and must not be normalised yet.
      const holdCR = buffer.endsWith("\r") ? 1 : 0;
      let scannable = normalizeNewlines(buffer.slice(0, buffer.length - holdCR));
      buffer = holdCR ? "\r" : "";

      for (;;) {
        const idx = scannable.indexOf("\n\n");
        if (idx === -1) break;
        const parsed = parseEventBlock(scannable.slice(0, idx));
        scannable = scannable.slice(idx + 2);
        if (parsed) yield parsed;
      }
      buffer = scannable + buffer;
    }
    buffer += decoder.decode();
    const trimmed = buffer.trim();
    if (trimmed) {
      const parsed = parseEventBlock(normalizeNewlines(trimmed));
      if (parsed) yield parsed;
    }
  } finally {
    reader.releaseLock();
  }
}
