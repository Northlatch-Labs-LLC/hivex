/**
 * Wire-shape types for the HiveAPI Gateway's OpenAI-compatible surface.
 *
 * Every shape here mirrors what the gateway actually emits on the wire:
 * - chat chunks follow OpenAI `chat.completion.chunk` (see
 *   9router/open-sse/translator/concerns/chunk.js — `buildChunk()`),
 * - errors follow `{ error: { message, type, code } }` (see
 *   9router/open-sse/utils/error.js — `buildErrorBody()` and
 *   9router/open-sse/config/errorConfig.js — `ERROR_TYPES`),
 * - models / embeddings / images follow the standard OpenAI objects,
 * - search follows the gateway's unified `/v1/search` response.
 */

// ---------------------------------------------------------------------------
// Chat — shared
// ---------------------------------------------------------------------------

export interface Usage {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  /** Present when the upstream provider reports cache tokens (gateway buildUsage()). */
  prompt_tokens_details?: {
    cached_tokens?: number;
    cache_creation_tokens?: number;
  };
  /** Present when the upstream provider reports reasoning tokens (gateway buildUsage()). */
  completion_tokens_details?: {
    reasoning_tokens?: number;
  };
}

export interface ChatFunctionDef {
  name: string;
  description?: string;
  parameters?: Record<string, unknown>;
  strict?: boolean;
}

export interface ChatToolDef {
  type: "function";
  function: ChatFunctionDef;
}

export interface ToolCall {
  id: string;
  type: "function";
  function: {
    name: string;
    arguments: string;
  };
}

export type ChatFinishReason =
  | "stop"
  | "length"
  | "tool_calls"
  | "content_filter"
  | (string & {});

// ---------------------------------------------------------------------------
// Chat — request
// ---------------------------------------------------------------------------

export interface ChatCompletionTextPart {
  type: "text";
  text: string;
}

export interface ChatCompletionImagePart {
  type: "image_url";
  image_url: { url: string; detail?: "auto" | "low" | "high" };
}

export type ChatContentPart = ChatCompletionTextPart | ChatCompletionImagePart;

export interface SystemMessage {
  role: "system";
  content: string | ChatContentPart[] | null;
}

export interface UserMessage {
  role: "user";
  content: string | ChatContentPart[] | null;
}

export interface AssistantMessage {
  role: "assistant";
  content: string | null;
  tool_calls?: ToolCall[];
}

export interface ToolMessage {
  role: "tool";
  tool_call_id: string;
  content: string | null;
}

export type ChatMessage =
  | SystemMessage
  | UserMessage
  | AssistantMessage
  | ToolMessage;

/** Gateway-recognised tool_choice forms (OpenAI semantics). */
export type ToolChoice =
  | "none"
  | "auto"
  | "required"
  | { type: "function"; function: { name: string } };

/**
 * POST /v1/chat/completions body. The gateway accepts the full OpenAI
 * parameter surface; the fields below are the ones it explicitly documents
 * and translates. Unknown extras pass through untouched.
 */
export interface ChatCompletionParams {
  model: string;
  messages: ChatMessage[];
  stream?: boolean;
  /** OpenAI streaming option — ask the gateway to append a usage frame. */
  stream_options?: { include_usage?: boolean };
  tools?: ChatToolDef[];
  tool_choice?: ToolChoice;
  temperature?: number;
  top_p?: number;
  max_tokens?: number;
  max_completion_tokens?: number;
  reasoning_effort?: "minimal" | "low" | "medium" | "high" | (string & {});
  stop?: string | string[];
  [key: string]: unknown;
}

// ---------------------------------------------------------------------------
// Chat — non-streaming response
// ---------------------------------------------------------------------------

export interface ChatCompletionChoice {
  index: number;
  message: AssistantMessage;
  finish_reason: string | null;
}

export interface ChatCompletion {
  id: string;
  object: "chat.completion";
  created: number;
  model: string;
  choices: ChatCompletionChoice[];
  usage?: Usage;
}

// ---------------------------------------------------------------------------
// Chat — streaming response
// ---------------------------------------------------------------------------

/**
 * A tool_calls delta fragment, exactly as the gateway emits it
 * (see 9router/open-sse/translator/response/claude-to-openai.js):
 * the first fragment carries {index, id, type, function:{name, arguments:""}}
 * and each argument fragment repeats the index (and id) with argument pieces.
 */
export interface ToolCallDelta {
  index: number;
  id?: string;
  type?: string;
  function?: {
    name?: string;
    arguments?: string;
  };
}

export interface ChatChunkDelta {
  role?: string;
  content?: string | null;
  /** Some providers emit chain-of-thought under reasoning_content. */
  reasoning_content?: string;
  tool_calls?: ToolCallDelta[];
}

export interface ChatCompletionChunkChoice {
  index: number;
  delta: ChatChunkDelta;
  finish_reason: string | null;
}

export interface ChatCompletionChunk {
  id: string;
  object: "chat.completion.chunk";
  created: number;
  model: string;
  choices: ChatCompletionChunkChoice[];
  /** Present on the final frame when stream_options.include_usage was sent. */
  usage?: Usage | null;
}

/** The folded result of one streamed assistant turn. */
export interface FinalAssistantMessage {
  id: string | null;
  model: string | null;
  role: "assistant";
  content: string;
  reasoning_content: string;
  tool_calls: ToolCall[];
  finish_reason: string | null;
  usage: Usage | null;
}

// ---------------------------------------------------------------------------
// Models — GET /v1/models
// ---------------------------------------------------------------------------

/**
 * One model entry. The gateway also emits optional capability fields
 * (capabilities, context_length, max_completion_tokens, kind) depending on
 * the provider — see 9router/src/app/api/v1/models/route.js.
 */
export interface ModelInfo {
  id: string;
  object: "model";
  owned_by: string;
  capabilities?: Record<string, unknown>;
  context_length?: number;
  max_completion_tokens?: number;
  kind?: string;
  [key: string]: unknown;
}

export interface ModelsResponse {
  object: "list";
  data: ModelInfo[];
}

// ---------------------------------------------------------------------------
// Embeddings — POST /v1/embeddings
// ---------------------------------------------------------------------------

export interface EmbeddingsParams {
  model: string;
  input: string | string[];
  encoding_format?: "float" | "base64";
  dimensions?: number;
}

export interface Embedding {
  object: "embedding";
  index: number;
  embedding: number[] | string;
}

export interface EmbeddingsResponse {
  object: "list";
  model: string;
  data: Embedding[];
  usage: {
    prompt_tokens: number;
    total_tokens: number;
  };
}

// ---------------------------------------------------------------------------
// Images — POST /v1/images/generations
// ---------------------------------------------------------------------------

export interface ImageGenerationParams {
  model: string;
  prompt: string;
  n?: number;
  size?: string;
  quality?: string;
  style?: string;
  response_format?: "url" | "b64_json";
}

export interface ImageGeneration {
  url?: string;
  b64_json?: string;
  revised_prompt?: string;
}

export interface ImageGenerationResponse {
  created: number;
  data: ImageGeneration[];
}

// ---------------------------------------------------------------------------
// Audio — POST /v1/audio/speech (TTS)
// ---------------------------------------------------------------------------

export interface SpeechParams {
  /** Voice/model ID from /v1/models/tts (e.g. "openai/tts-1", "el/<voice_id>"). */
  model: string;
  input: string;
}

/** Returned by /v1/audio/speech?response_format=json — the default response is raw audio bytes. */
export interface SpeechJsonResponse {
  audio: string;
  format: string;
}

// ---------------------------------------------------------------------------
// Audio — POST /v1/audio/transcriptions (STT, multipart/form-data)
// ---------------------------------------------------------------------------

export interface TranscriptionParams {
  /** STT model ID from /v1/models/stt (e.g. "openai/whisper-1"). */
  model: string;
  /** Audio bytes: a Blob, File, or raw Uint8Array (use `filename` with the latter). */
  file: Blob | File | Uint8Array;
  filename?: string;
  language?: string;
  prompt?: string;
  temperature?: number;
  /** OpenAI-style response_format: json (default) / text / verbose_json / srt / vtt. */
  response_format?: string;
}

export interface TranscriptionResponse {
  text: string;
  language?: string;
  duration?: number;
  segments?: unknown[];
}

// ---------------------------------------------------------------------------
// Web search — POST /v1/search
// ---------------------------------------------------------------------------

export interface SearchParams {
  /** Search model/provider id from /v1/models/web (e.g. "tavily", "tavily/search", a search combo). */
  model: string;
  query: string;
  max_results?: number;
  search_type?: string;
  country?: string;
  language?: string;
  time_range?: string;
  domain_filter?: string[];
  provider_options?: Record<string, unknown>;
}

export interface SearchResult {
  title?: string | null;
  url?: string | null;
  display_url?: string | null;
  snippet?: string | null;
  position?: number;
  score?: number;
  published_at?: string | null;
  favicon_url?: string | null;
  content?: string | null;
  metadata?: Record<string, unknown> | null;
  citation?: Record<string, unknown> | null;
}

export interface SearchUsage {
  queries_used?: number;
  search_cost_usd?: number | null;
  provider_credits_used?: number | null;
}

export interface SearchMetrics {
  response_time_ms?: number;
  upstream_latency_ms?: number;
  total_results_available?: number;
}

export interface SearchResponse {
  provider: string;
  query: string;
  results: SearchResult[];
  answer?: string | null;
  usage?: SearchUsage | null;
  metrics?: SearchMetrics | null;
  pagination?: {
    has_more?: boolean;
    next_cursor?: string | null;
  } | null;
  errors?: unknown[];
}
