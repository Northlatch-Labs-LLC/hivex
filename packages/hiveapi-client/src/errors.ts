/**
 * Classified errors for the HiveAPI Gateway surface.
 *
 * The gateway answers failures with `{ error: { message, type, code } }`
 * (9router/open-sse/utils/error.js) plus these status semantics:
 *
 * - 401/402/403 — auth/billing/permission rejections
 * - 429         — rate limited
 * - 503         — the gateway's "all upstream accounts exhausted" answer,
 *                 always paired with a `Retry-After` header
 *                 (9router/open-sse/utils/error.js — unavailableResponse)
 * - other 5xx   — upstream/server failures
 * - other 4xx   — invalid requests
 */

export type HiveAPIErrorKind =
  | "unreachable"
  | "unauthorized"
  | "rate_limited"
  | "server_error"
  | "invalid_request";

/** Map an HTTP status code to a stable error kind. */
export function classifyStatus(status: number): HiveAPIErrorKind {
  if (status === 401 || status === 402 || status === 403) return "unauthorized";
  if (status === 429 || status === 503) return "rate_limited";
  if (status >= 500) return "server_error";
  return "invalid_request";
}

/** Map the gateway's error.type strings (errorConfig.js ERROR_TYPES) to kinds. */
export function classifyErrorType(type: string): HiveAPIErrorKind {
  switch (type) {
    case "authentication_error":
    case "billing_error":
    case "permission_error":
      return "unauthorized";
    case "rate_limit_error":
      return "rate_limited";
    case "server_error":
      return "server_error";
    default:
      return "invalid_request";
  }
}

export interface HiveAPIErrorInit {
  kind: HiveAPIErrorKind;
  message: string;
  /** HTTP status; null when the request never got a response (unreachable). */
  status?: number | null;
  /** The gateway's error.type field, when present. */
  type?: string | null;
  /** The gateway's error.code field, when present. */
  code?: string | null;
  /** Parsed Retry-After header (seconds) — the gateway sets it on 429/503. */
  retryAfterSeconds?: number | null;
  url?: string;
  bodyText?: string;
  cause?: unknown;
}

export class HiveAPIError extends Error {
  override readonly name = "HiveAPIError";
  readonly kind: HiveAPIErrorKind;
  readonly status: number | null;
  readonly type: string | null;
  readonly code: string | null;
  readonly retryAfterSeconds: number | null;
  readonly url: string | undefined;
  readonly bodyText: string | undefined;

  constructor(init: HiveAPIErrorInit) {
    super(init.message, { cause: init.cause });
    this.kind = init.kind;
    this.status = init.status ?? null;
    this.type = init.type ?? null;
    this.code = init.code ?? null;
    this.retryAfterSeconds = init.retryAfterSeconds ?? null;
    this.url = init.url;
    this.bodyText = init.bodyText;
  }

  /** True when the request failed before any HTTP response arrived. */
  get isUnreachable(): boolean {
    return this.kind === "unreachable";
  }
}

export function isHiveAPIError(value: unknown): value is HiveAPIError {
  return value instanceof HiveAPIError;
}

/** Parse a Retry-After header (seconds form; the gateway emits seconds). */
export function parseRetryAfter(header: string | null): number | null {
  if (!header) return null;
  const seconds = Number(header);
  if (Number.isFinite(seconds) && seconds >= 0) return Math.ceil(seconds);
  // HTTP-date form — fall back to a conservative 0 (treat as retryable now).
  const asDate = Date.parse(header);
  if (!Number.isNaN(asDate)) return Math.max(0, Math.ceil((asDate - Date.now()) / 1000));
  return null;
}
