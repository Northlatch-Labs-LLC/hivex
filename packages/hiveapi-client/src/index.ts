/**
 * @hiveapi/client — typed client for the HiveAPI Gateway.
 *
 * @packageDocumentation
 */

export {
  HiveAPIClient,
  type HiveAPIClientOptions,
  type RequestOptions,
  type FetchLike,
} from "./client.ts";

export {
  HiveAPIError,
  classifyStatus,
  classifyErrorType,
  isHiveAPIError,
  parseRetryAfter,
  type HiveAPIErrorKind,
  type HiveAPIErrorInit,
} from "./errors.ts";

export {
  ChatStream,
  chunksFromResponse,
  collectFinalMessage,
  completionToChunks,
  accumulateToolCallFragment,
  buildChunk,
} from "./stream.ts";

export {
  iterateSSE,
  SSE_DONE,
  type SSEEvent,
} from "./sse.ts";

export {
  normalizeBaseURL,
  formatBaseURL,
  buildRequestURL,
  type NormalizedBase,
} from "./url.ts";

export type {
  // Chat
  Usage,
  ChatFunctionDef,
  ChatToolDef,
  ToolCall,
  ToolCallDelta,
  ToolChoice,
  ChatFinishReason,
  ChatContentPart,
  ChatCompletionTextPart,
  ChatCompletionImagePart,
  SystemMessage,
  UserMessage,
  AssistantMessage,
  ToolMessage,
  ChatMessage,
  ChatCompletionParams,
  ChatCompletionChoice,
  ChatCompletion,
  ChatChunkDelta,
  ChatCompletionChunkChoice,
  ChatCompletionChunk,
  FinalAssistantMessage,
  // Models
  ModelInfo,
  ModelsResponse,
  // Embeddings
  EmbeddingsParams,
  Embedding,
  EmbeddingsResponse,
  // Images
  ImageGenerationParams,
  ImageGeneration,
  ImageGenerationResponse,
  // Audio
  SpeechParams,
  SpeechJsonResponse,
  TranscriptionParams,
  TranscriptionResponse,
  // Search
  SearchParams,
  SearchResult,
  SearchResponse,
  SearchUsage,
  SearchMetrics,
} from "./types.ts";
