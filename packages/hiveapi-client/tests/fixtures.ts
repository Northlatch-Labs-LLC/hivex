/**
 * Realistic SSE fixtures mirroring the gateway's actual client-facing wire
 * format: `data: {...}\n\n` frames of OpenAI `chat.completion.chunk` objects
 * (9router/open-sse/translator/concerns/chunk.js — buildChunk), tool-call
 * fragment emission (claude-to-openai.js — first fragment carries
 * {index, id, type, function:{name, arguments:""}}, argument fragments repeat
 * the index/id and append argument pieces), the terminal usage frame sent
 * under stream_options.include_usage, and the `data: [DONE]\n\n` sentinel
 * (9router/open-sse/utils/sseConstants.js).
 */

const CHATCMPL = "chatcmpl-hiveapi-fix01";
const CREATED = 1758326400;
const MODEL = "openai/gpt-5";

function sse(payload: unknown): string {
  return `data: ${JSON.stringify(payload)}\n\n`;
}

function chunk(delta: Record<string, unknown>, finishReason: string | null) {
  return {
    id: CHATCMPL,
    object: "chat.completion.chunk",
    created: CREATED,
    model: MODEL,
    choices: [{ index: 0, delta, finish_reason: finishReason }],
  };
}

function usageChunk() {
  return {
    id: CHATCMPL,
    object: "chat.completion.chunk",
    created: CREATED,
    model: MODEL,
    choices: [{ index: 0, delta: {}, finish_reason: null }],
    usage: {
      prompt_tokens: 42,
      completion_tokens: 18,
      total_tokens: 60,
    },
  };
}

/** A plain-text streamed turn: role delta → 3 content deltas → stop → usage → [DONE]. */
export const chatTextFrames: string[] = [
  sse(chunk({ role: "assistant" }, null)),
  sse(chunk({ content: "Hello" }, null)),
  sse(chunk({ content: ", " }, null)),
  sse(chunk({ content: "world!" }, null)),
  sse(chunk({}, "stop")),
  sse(usageChunk()),
  "data: [DONE]\n\n",
];

/**
 * A streamed tool-call turn mirroring the gateway's fragment emission:
 * role → thinking marker + reasoning → tool-call head → 3 argument fragments
 * → finish_reason "tool_calls" → usage → [DONE].
 */
export const chatToolCallFrames: string[] = [
  sse(chunk({ role: "assistant" }, null)),
  sse(chunk({ reasoning_content: "Checking the weather service..." }, null)),
  sse(
    chunk(
      {
        tool_calls: [
          {
            index: 0,
            id: "toolu_hive01ABCdefGHIjklMNOpq",
            type: "function",
            function: { name: "get_weather", arguments: "" },
          },
        ],
      },
      null,
    ),
  ),
  sse(
    chunk(
      {
        tool_calls: [
          {
            index: 0,
            id: "toolu_hive01ABCdefGHIjklMNOpq",
            function: { arguments: '{"city":' },
          },
        ],
      },
      null,
    ),
  ),
  sse(
    chunk(
      {
        tool_calls: [
          {
            index: 0,
            id: "toolu_hive01ABCdefGHIjklMNOpq",
            function: { arguments: '"Hanoi",' },
          },
        ],
      },
      null,
    ),
  ),
  sse(
    chunk(
      {
        tool_calls: [
          {
            index: 0,
            id: "toolu_hive01ABCdefGHIjklMNOpq",
            function: { arguments: '"unit":"celsius"}' },
          },
        ],
      },
      null,
    ),
  ),
  sse(chunk({}, "tool_calls")),
  sse(usageChunk()),
  "data: [DONE]\n\n",
];

/** The arguments JSON the fragments above accumulate to. */
export const expectedToolCallArguments = '{"city":"Hanoi","unit":"celsius"}';

/** Mid-stream failure: the gateway emits an error frame then [DONE] (streamHelpers.js). */
export const chatErrorFrames: string[] = [
  sse(chunk({ role: "assistant" }, null)),
  sse(chunk({ content: "Hel" }, null)),
  sse({
    error: {
      message: "[429]: Rate limit exceeded (reset after 30s)",
      type: "rate_limit_error",
      code: "rate_limit_exceeded",
    },
  }),
  "data: [DONE]\n\n",
];

/** Parser edge cases: event field, comment/heartbeat line, multi-line data payload. */
export const parserEdgeCaseFrames: string[] = [
  ": keep-alive comment\n\n",
  `event: message\ndata: ${JSON.stringify({ hello: "line one" })}\n\n`,
  "data: first line\ndata: second line\n\n",
  "data: [DONE]\n\n",
];

/** CRLF variant of the text stream (some proxies rewrite line endings). */
export const chatTextCrlfFrames: string[] = chatTextFrames.map((frame) =>
  frame.replace(/\n/g, "\r\n"),
);

/** The non-streaming chat.completion shape (GET from the skill docs / handleChatCore). */
export const chatCompletionBody = {
  id: "chatcmpl-hiveapi-fix02",
  object: "chat.completion",
  created: CREATED,
  model: MODEL,
  choices: [
    {
      index: 0,
      message: { role: "assistant", content: "Hello from the HiveAPI Gateway!" },
      finish_reason: "stop",
    },
  ],
  usage: {
    prompt_tokens: 8,
    completion_tokens: 2,
    total_tokens: 10,
  },
};

/** Join wire frames into a single SSE body string. */
export function joinFrames(frames: string[]): string {
  return frames.join("");
}

/** Encode wire frames into bytes for a Response body / ReadableStream. */
export function framesToBytes(frames: string[]): Uint8Array {
  return new TextEncoder().encode(joinFrames(frames));
}

/** A ReadableStream that emits the frames in small pieces to exercise the parser. */
export function framesToChunkedStream(
  frames: string[],
  pieceSize = 7,
): ReadableStream<Uint8Array> {
  const bytes = framesToBytes(frames);
  let offset = 0;
  return new ReadableStream<Uint8Array>({
    pull(controller) {
      if (offset >= bytes.length) {
        controller.close();
        return;
      }
      const end = Math.min(offset + pieceSize, bytes.length);
      controller.enqueue(bytes.slice(offset, end));
      offset = end;
    },
  });
}

/** A ReadableStream emitting the frames as one chunk. */
export function framesToStream(frames: string[]): ReadableStream<Uint8Array> {
  const bytes = framesToBytes(frames);
  return new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(bytes);
      controller.close();
    },
  });
}

/** Gateway-style error body (9router/open-sse/utils/error.js — buildErrorBody). */
export function gatewayErrorBody(status: number, message: string): Record<string, unknown> {
  const types: Record<number, { type: string; code: string }> = {
    400: { type: "invalid_request_error", code: "bad_request" },
    401: { type: "authentication_error", code: "invalid_api_key" },
    404: { type: "invalid_request_error", code: "model_not_found" },
    429: { type: "rate_limit_error", code: "rate_limit_exceeded" },
    500: { type: "server_error", code: "internal_server_error" },
    502: { type: "server_error", code: "bad_gateway" },
    504: { type: "server_error", code: "gateway_timeout" },
  };
  const info = types[status] ?? { type: "server_error", code: "internal_server_error" };
  return { error: { message, type: info.type, code: info.code } };
}
