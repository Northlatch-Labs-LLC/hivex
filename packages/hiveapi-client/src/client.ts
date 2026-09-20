/**
 * HiveAPIClient — typed client for the HiveAPI Gateway's unified inference
 * surface ({base}/v1), mirroring the real routes:
 *
 * - POST /v1/chat/completions   (OpenAI chat, SSE streaming, tools, usage)
 * - GET  /v1/models
 * - POST /v1/embeddings
 * - POST /v1/images/generations (json by default, ?response_format=binary)
 * - POST /v1/audio/speech       (raw bytes by default, ?response_format=json)
 * - POST /v1/audio/transcriptions (multipart/form-data, Whisper-compatible)
 * - POST /v1/search             (unified web search)
 *
 * Zero runtime dependencies: global fetch + Web Streams only.
 */

import {
  HiveAPIError,
  classifyStatus,
  parseRetryAfter,
} from "./errors.ts";
import { ChatStream } from "./stream.ts";
import type {
  ChatCompletion,
  ChatCompletionParams,
  EmbeddingsParams,
  EmbeddingsResponse,
  ImageGenerationParams,
  ImageGenerationResponse,
  ModelsResponse,
  SearchParams,
  SearchResponse,
  SpeechJsonResponse,
  SpeechParams,
  TranscriptionParams,
  TranscriptionResponse,
} from "./types.ts";
import {
  buildRequestURL,
  formatBaseURL,
  normalizeBaseURL,
  type NormalizedBase,
} from "./url.ts";

/**
 * Minimal structural type for fetch — accepts plain async functions in tests
 * and runtimes where the global fetch carries extra statics (e.g. Bun's
 * `fetch.preconnect`).
 */
export type FetchLike = (
  input: string | URL | Request,
  init?: RequestInit,
) => Promise<Response>;

export interface HiveAPIClientOptions {
  /**
   * Gateway base URL. `/v1` is auto-appended when missing, and an existing
   * query string is preserved on every request, e.g.
   * "https://gw.example" → "https://gw.example/v1",
   * "https://gw.example?token=abc" → "https://gw.example/v1?token=abc".
   */
  baseURL: string;
  /**
   * Gateway API key (Bearer). Optional — a local gateway with
   * requireApiKey=false accepts anonymous requests.
   */
  apiKey?: string;
  /** Fetch implementation override (tests, proxies). Defaults to global fetch. */
  fetchImpl?: FetchLike;
}

export interface RequestOptions {
  /** Abort the underlying request. */
  signal?: AbortSignal;
}

interface RouteOptions {
  method: "GET" | "POST";
  route: string;
  body?: unknown;
  form?: FormData;
  signal?: AbortSignal;
}

export class HiveAPIClient {
  readonly #base: NormalizedBase;
  readonly #apiKey: string | undefined;
  readonly #fetch: FetchLike;

  readonly chat: ChatResource;
  readonly models: ModelsResource;
  readonly embeddings: EmbeddingsResource;
  readonly images: ImagesResource;
  readonly audio: AudioResource;
  readonly search: SearchResource;

  constructor(options: HiveAPIClientOptions) {
    this.#base = normalizeBaseURL(options.baseURL);
    this.#apiKey = options.apiKey;
    this.#fetch = options.fetchImpl ?? globalThis.fetch;

    this.chat = new ChatResource(this);
    this.models = new ModelsResource(this);
    this.embeddings = new EmbeddingsResource(this);
    this.images = new ImagesResource(this);
    this.audio = new AudioResource(this);
    this.search = new SearchResource(this);
  }

  /** The normalised base URL (with /v1 and any query string). */
  get baseURL(): string {
    return formatBaseURL(this.#base);
  }

  #url(route: string): string {
    return buildRequestURL(this.#base, route);
  }

  #headers(json: boolean): Record<string, string> {
    const headers: Record<string, string> = {};
    if (this.#apiKey) headers.Authorization = `Bearer ${this.#apiKey}`;
    if (json) headers["Content-Type"] = "application/json";
    return headers;
  }

  async #send(opts: RouteOptions): Promise<Response> {
    const url = this.#url(opts.route);
    let response: Response;
    try {
      response = await this.#fetch(url, {
        method: opts.method,
        headers: this.#headers(opts.body !== undefined),
        body: opts.form ?? (opts.body !== undefined ? JSON.stringify(opts.body) : undefined),
        signal: opts.signal,
      });
    } catch (cause) {
      if (cause instanceof Error && cause.name === "AbortError") throw cause;
      throw new HiveAPIError({
        kind: "unreachable",
        message: `Could not reach the HiveAPI Gateway at ${formatBaseURL(this.#base)}`,
        url,
        cause,
      });
    }
    if (!response.ok) throw await HiveAPIClient.#errorFromResponse(response, url);
    return response;
  }

  static async #errorFromResponse(response: Response, url: string): Promise<HiveAPIError> {
    const text = await response.text().catch(() => "");
    let message = text || response.statusText || `HTTP ${response.status}`;
    let type: string | null = null;
    let code: string | null = null;
    try {
      const parsed: unknown = JSON.parse(text);
      if (typeof parsed === "object" && parsed !== null) {
        const { error } = parsed as { error?: unknown };
        if (typeof error === "string") {
          message = error;
        } else if (typeof error === "object" && error !== null) {
          const err = error as { message?: unknown; type?: unknown; code?: unknown };
          if (typeof err.message === "string") message = err.message;
          if (typeof err.type === "string") type = err.type;
          if (typeof err.code === "string") code = err.code;
        } else {
          const { message: jsonMessage } = parsed as { message?: unknown };
          if (typeof jsonMessage === "string") message = jsonMessage;
        }
      }
    } catch {
      // Non-JSON error body — keep the raw text as the message.
    }
    return new HiveAPIError({
      kind: classifyStatus(response.status),
      message,
      status: response.status,
      type,
      code,
      retryAfterSeconds: parseRetryAfter(response.headers.get("retry-after")),
      url,
      bodyText: text,
    });
  }

  async #json<T>(response: Response): Promise<T> {
    try {
      return (await response.json()) as T;
    } catch (cause) {
      throw new HiveAPIError({
        kind: "server_error",
        message: "Gateway returned invalid JSON",
        status: response.status,
        cause,
      });
    }
  }

  /** Direct HTTP surface for the portal/MCP package (also handy in apps). */
  async request(
    route: string,
    init?: { method?: "GET" | "POST"; body?: unknown; form?: FormData; signal?: AbortSignal },
  ): Promise<Response> {
    return this.#send({
      method: init?.method ?? "GET",
      route,
      body: init?.body,
      form: init?.form,
      signal: init?.signal,
    });
  }
}

// ---------------------------------------------------------------------------
// Resources
// ---------------------------------------------------------------------------

class ChatResource {
  readonly completions: ChatCompletionsResource;

  constructor(client: HiveAPIClient) {
    this.completions = new ChatCompletionsResource(client);
  }
}

class ChatCompletionsResource {
  readonly #client: HiveAPIClient;

  constructor(client: HiveAPIClient) {
    this.#client = client;
  }

  /**
   * POST /v1/chat/completions.
   *
   * With `stream: true` the returned ChatStream is an async iterator over
   * gateway SSE chunks ([DONE] sentinel handled) — `finalMessage()` folds
   * content, tool-call fragments and usage. If the gateway answers JSON
   * instead of an event stream, the stream falls back to synthetic chunks.
   */
  create(
    params: ChatCompletionParams & { stream: true },
    options?: RequestOptions,
  ): Promise<ChatStream>;
  create(
    params: ChatCompletionParams & { stream?: false },
    options?: RequestOptions,
  ): Promise<ChatCompletion>;
  create(
    params: ChatCompletionParams,
    options?: RequestOptions,
  ): Promise<ChatStream | ChatCompletion>;
  async create(
    params: ChatCompletionParams,
    options?: RequestOptions,
  ): Promise<ChatStream | ChatCompletion> {
    const response = await this.#client.request("chat/completions", {
      method: "POST",
      body: params,
      signal: options?.signal,
    });
    if (params.stream === true) return ChatStream.fromResponse(response);
    const contentType = response.headers.get("content-type") ?? "";
    if (contentType.includes("text/event-stream")) {
      // Server ignored stream:false — consume the SSE stream and fold it.
      const stream = ChatStream.fromResponse(response);
      const final = await stream.finalMessage();
      return {
        id: final.id ?? "chatcmpl-unknown",
        object: "chat.completion",
        created: Math.floor(Date.now() / 1000),
        model: final.model ?? params.model,
        choices: [
          {
            index: 0,
            message: {
              role: "assistant",
              content: final.content,
              ...(final.tool_calls.length > 0 ? { tool_calls: final.tool_calls } : {}),
            },
            finish_reason: final.finish_reason,
          },
        ],
        ...(final.usage ? { usage: final.usage } : {}),
      };
    }
    return (await response.json()) as ChatCompletion;
  }
}

class ModelsResource {
  readonly #client: HiveAPIClient;

  constructor(client: HiveAPIClient) {
    this.#client = client;
  }

  /** GET /v1/models — OpenAI-compatible model list. */
  async list(options?: RequestOptions): Promise<ModelsResponse> {
    const response = await this.#client.request("models", { signal: options?.signal });
    return (await response.json()) as ModelsResponse;
  }
}

class EmbeddingsResource {
  readonly #client: HiveAPIClient;

  constructor(client: HiveAPIClient) {
    this.#client = client;
  }

  /** POST /v1/embeddings. */
  async create(params: EmbeddingsParams, options?: RequestOptions): Promise<EmbeddingsResponse> {
    const response = await this.#client.request("embeddings", {
      method: "POST",
      body: params,
      signal: options?.signal,
    });
    return (await response.json()) as EmbeddingsResponse;
  }
}

class ImagesResource {
  readonly #client: HiveAPIClient;

  constructor(client: HiveAPIClient) {
    this.#client = client;
  }

  /** POST /v1/images/generations — JSON response (url or b64_json). */
  generate(
    params: ImageGenerationParams,
    options?: RequestOptions & { binary?: false },
  ): Promise<ImageGenerationResponse>;
  /** POST /v1/images/generations?response_format=binary — raw image bytes. */
  generate(
    params: ImageGenerationParams,
    options: RequestOptions & { binary: true },
  ): Promise<ArrayBuffer>;
  async generate(
    params: ImageGenerationParams,
    options?: RequestOptions & { binary?: boolean },
  ): Promise<ImageGenerationResponse | ArrayBuffer> {
    const route = options?.binary ? "images/generations?response_format=binary" : "images/generations";
    const response = await this.#client.request(route, {
      method: "POST",
      body: params,
      signal: options?.signal,
    });
    if (options?.binary) return response.arrayBuffer();
    return (await response.json()) as ImageGenerationResponse;
  }
}

class AudioResource {
  readonly #client: HiveAPIClient;

  constructor(client: HiveAPIClient) {
    this.#client = client;
  }

  /**
   * POST /v1/audio/speech — text-to-speech.
   *
   * Default: raw audio bytes (ArrayBuffer, gateway answers audio/mp3).
   * With `responseFormat: "json"`: `{ audio: base64, format }`.
   */
  speech(
    params: SpeechParams,
    options: RequestOptions & { responseFormat: "json" },
  ): Promise<SpeechJsonResponse>;
  speech(
    params: SpeechParams,
    options?: RequestOptions & { responseFormat?: string },
  ): Promise<ArrayBuffer>;
  async speech(
    params: SpeechParams,
    options?: RequestOptions & { responseFormat?: string },
  ): Promise<ArrayBuffer | SpeechJsonResponse> {
    const route = options?.responseFormat
      ? `audio/speech?response_format=${encodeURIComponent(options.responseFormat)}`
      : "audio/speech";
    const response = await this.#client.request(route, {
      method: "POST",
      body: params,
      signal: options?.signal,
    });
    if (options?.responseFormat === "json") {
      return (await response.json()) as SpeechJsonResponse;
    }
    return response.arrayBuffer();
  }

  /** POST /v1/audio/transcriptions (multipart/form-data, Whisper-compatible). */
  transcriptions(
    params: TranscriptionParams & { response_format?: "json" | "verbose_json" },
    options?: RequestOptions,
  ): Promise<TranscriptionResponse>;
  transcriptions(
    params: TranscriptionParams & { response_format: "text" | "srt" | "vtt" },
    options?: RequestOptions,
  ): Promise<string>;
  async transcriptions(
    params: TranscriptionParams,
    options?: RequestOptions,
  ): Promise<TranscriptionResponse | string> {
    const form = new FormData();
    form.append("model", params.model);
    const file = params.file instanceof Blob ? params.file : new Blob([params.file]);
    const fallbackName = params.file instanceof File ? params.file.name : undefined;
    form.append("file", file, params.filename ?? fallbackName ?? "audio.mp3");
    if (params.language !== undefined) form.append("language", params.language);
    if (params.prompt !== undefined) form.append("prompt", params.prompt);
    if (params.temperature !== undefined) form.append("temperature", String(params.temperature));
    if (params.response_format !== undefined) form.append("response_format", params.response_format);

    const response = await this.#client.request("audio/transcriptions", {
      method: "POST",
      form,
      signal: options?.signal,
    });
    const contentType = response.headers.get("content-type") ?? "";
    const format = params.response_format ?? "json";
    if (contentType.includes("application/json") || format === "json" || format === "verbose_json") {
      return (await response.json()) as TranscriptionResponse;
    }
    return response.text();
  }
}

class SearchResource {
  readonly #client: HiveAPIClient;

  constructor(client: HiveAPIClient) {
    this.#client = client;
  }

  /** POST /v1/search — unified web search. */
  async create(params: SearchParams, options?: RequestOptions): Promise<SearchResponse> {
    const response = await this.#client.request("search", {
      method: "POST",
      body: params,
      signal: options?.signal,
    });
    return (await response.json()) as SearchResponse;
  }
}
