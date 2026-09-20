/**
 * Streaming chat helpers: turn a gateway SSE response into an async iterator
 * of OpenAI chunks, fold chunk deltas into a final assistant message, and
 * fall back to a single synthetic chunk when the server answers JSON to a
 * `stream:true` request.
 */

import type {
  ChatCompletion,
  ChatCompletionChunk,
  ChatCompletionChunkChoice,
  FinalAssistantMessage,
  ToolCall,
  ToolCallDelta,
  Usage,
} from "./types.ts";
import { HiveAPIError, classifyErrorType } from "./errors.ts";
import { iterateSSE, SSE_DONE } from "./sse.ts";

const DEFAULT_STREAM_ID = "chatcmpl-stream";

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

/**
 * Fold one tool-call fragment into the accumulator (mirrors how the gateway
 * emits fragments — see 9router/open-sse/translator/response/claude-to-openai.js:
 * the first fragment sets {index, id, type, function:{name, arguments:""}},
 * later fragments repeat index/id and append argument pieces).
 */
export function accumulateToolCallFragment(
  acc: Map<number, ToolCall>,
  delta: ToolCallDelta,
): void {
  const index = delta.index ?? 0;
  const existing = acc.get(index);
  const id = delta.id ?? existing?.id ?? `call_${index}`;
  const name = delta.function?.name ?? existing?.function.name ?? "";
  const argsPiece = delta.function?.arguments ?? "";
  const args = (existing?.function.arguments ?? "") + argsPiece;
  acc.set(index, { id, type: "function", function: { name, arguments: args } });
}

function toolCallsFromMessage(message: { tool_calls?: ToolCall[] }): ToolCallDelta[] {
  return (message.tool_calls ?? []).map((tc, i) => ({
    index: i,
    id: tc.id,
    type: tc.type,
    function: { name: tc.function.name, arguments: tc.function.arguments },
  }));
}

/** Build a chat.completion.chunk shape (9router concerns/chunk.js — buildChunk). */
export function buildChunk(
  meta: Pick<ChatCompletionChunk, "id" | "created" | "model">,
  delta: ChatCompletionChunkChoice["delta"],
  finishReason: string | null,
  usage?: Usage | null,
): ChatCompletionChunk {
  const chunk: ChatCompletionChunk = {
    id: meta.id,
    object: "chat.completion.chunk",
    created: meta.created,
    model: meta.model,
    choices: [{ index: 0, delta, finish_reason: finishReason }],
  };
  if (usage !== undefined) chunk.usage = usage;
  return chunk;
}

/** Split a non-streaming chat.completion into chunk-shaped frames (fallback). */
export function* completionToChunks(completion: ChatCompletion): Generator<ChatCompletionChunk> {
  const meta = {
    id: completion.id || DEFAULT_STREAM_ID,
    created: completion.created,
    model: completion.model,
  };
  const choice = completion.choices[0];
  const message = choice?.message;
  const delta: ChatCompletionChunkChoice["delta"] = {
    role: "assistant",
    content: message?.content ?? null,
  };
  if (message?.tool_calls?.length) delta.tool_calls = toolCallsFromMessage(message);
  yield buildChunk(meta, delta, choice?.finish_reason ?? null);
  if (completion.usage) {
    // Mirror the gateway's include_usage terminal frame: empty delta + usage.
    yield buildChunk(meta, {}, null, completion.usage);
  }
}

interface ErrorFrame {
  error: { message?: unknown; type?: unknown; code?: unknown };
}

function errorFromFrame(frame: ErrorFrame): HiveAPIError {
  const err = asRecord(frame.error);
  const message = typeof err?.message === "string" ? err.message : "Gateway stream error";
  const type = typeof err?.type === "string" ? err.type : null;
  return new HiveAPIError({
    kind: type ? classifyErrorType(type) : "server_error",
    message,
    status: null,
    type,
    code: typeof err?.code === "string" ? err.code : null,
  });
}

/**
 * Read a chat response as a stream of chunks.
 *
 * - SSE responses: parse frames; stop at the [DONE] sentinel; mid-stream
 *   `data: {"error":{...}}` frames (9router streamHelpers.js —
 *   buildStreamErrorBytes) become HiveAPIError.
 * - JSON responses to a stream:true request: non-stream fallback — the whole
 *   chat.completion is converted to synthetic chunks so consumers keep one
 *   consumption path.
 */
export async function* chunksFromResponse(
  response: Response,
): AsyncGenerator<ChatCompletionChunk> {
  const contentType = response.headers.get("content-type") ?? "";
  if (!contentType.includes("text/event-stream")) {
    let completion: unknown;
    try {
      completion = await response.json();
    } catch (cause) {
      throw new HiveAPIError({
        kind: "server_error",
        message: "Gateway returned a non-SSE body that is not valid JSON",
        cause,
      });
    }
    const body = asRecord(completion);
    if (body?.error) {
      throw errorFromFrame({ error: body.error });
    }
    yield* completionToChunks(completion as ChatCompletion);
    return;
  }

  if (!response.body) {
    throw new HiveAPIError({
      kind: "server_error",
      message: "Gateway returned an event-stream response with no body",
    });
  }

  for await (const event of iterateSSE(response.body)) {
    if (event.data === SSE_DONE) return;
    let parsed: unknown;
    try {
      parsed = JSON.parse(event.data);
    } catch {
      continue; // Ignore keep-alive / non-JSON frames.
    }
    const frame = asRecord(parsed);
    if (frame?.error) {
      throw errorFromFrame({ error: frame.error });
    }
    if (frame?.object === "chat.completion.chunk" && Array.isArray(frame.choices)) {
      yield parsed as ChatCompletionChunk;
    }
  }
}

/** Fold a chunk stream into the final assistant message (content + tool calls + usage). */
export async function collectFinalMessage(
  chunks: AsyncIterable<ChatCompletionChunk>,
): Promise<FinalAssistantMessage> {
  let id: string | null = null;
  let model: string | null = null;
  let content = "";
  let reasoning = "";
  let finishReason: string | null = null;
  let usage: Usage | null = null;
  const toolCalls = new Map<number, ToolCall>();

  for await (const chunk of chunks) {
    id = chunk.id ?? id;
    model = chunk.model ?? model;
    if (chunk.usage) usage = chunk.usage;
    for (const choice of chunk.choices) {
      const delta = choice.delta ?? {};
      if (typeof delta.content === "string") content += delta.content;
      if (typeof delta.reasoning_content === "string") reasoning += delta.reasoning_content;
      for (const fragment of delta.tool_calls ?? []) {
        accumulateToolCallFragment(toolCalls, fragment);
      }
      if (choice.finish_reason) finishReason = choice.finish_reason;
    }
  }

  return {
    id,
    model,
    role: "assistant",
    content,
    reasoning_content: reasoning,
    tool_calls: [...toolCalls.keys()].sort((a, b) => a - b).map((k) => toolCalls.get(k) as ToolCall),
    finish_reason: finishReason,
    usage,
  };
}

/**
 * Async-iterable wrapper over a streaming chat response with a one-shot
 * `finalMessage()` convenience that consumes the stream and folds it.
 */
export class ChatStream implements AsyncIterable<ChatCompletionChunk> {
  readonly #chunks: AsyncGenerator<ChatCompletionChunk>;

  constructor(chunks: AsyncGenerator<ChatCompletionChunk>) {
    this.#chunks = chunks;
  }

  static fromResponse(response: Response): ChatStream {
    return new ChatStream(chunksFromResponse(response));
  }

  [Symbol.asyncIterator](): AsyncGenerator<ChatCompletionChunk> {
    return this.#chunks;
  }

  /** Consume the whole stream and return the folded assistant message. */
  async finalMessage(): Promise<FinalAssistantMessage> {
    return collectFinalMessage(this.#chunks);
  }

  async toArray(): Promise<ChatCompletionChunk[]> {
    const out: ChatCompletionChunk[] = [];
    for await (const chunk of this.#chunks) out.push(chunk);
    return out;
  }
}
