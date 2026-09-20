/**
 * HTTP helpers for the gateway's customer-portal and admin key-store routes.
 *
 * These live OUTSIDE /v1, at the gateway root:
 *
 * - GET  /api/portal/usage       (portal session cookie — 9router portal/usage/route.js)
 * - GET  /api/portal/entitlement (portal session cookie — 9router portal/entitlement/route.js)
 * - POST /api/portal/turns       (portal session OR Bearer account key — 9router portal/turns/route.js)
 * - POST /api/keys               (dashboard admin auth — 9router api/keys/route.js)
 * - DELETE /api/keys/{id}        (dashboard admin auth — 9router api/keys/[id]/route.js)
 */

import type { HiveEnv } from "./env.ts";

// ---------------------------------------------------------------------------
// Response shapes (as returned by the gateway)
// ---------------------------------------------------------------------------

export interface PortalKeyUsage {
  id: string;
  name: string | null;
  maskedKey: string | null;
  isActive: boolean;
  requests: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
}

export interface UsageTotals {
  requests: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
}

export interface DailyUsage extends UsageTotals {
  date: string;
}

/** GET /api/portal/usage — 9router/src/lib/portal/usage.js getAccountMonthlyUsage(). */
export interface PortalUsageResponse {
  monthStart: string;
  tier: string;
  keys: PortalKeyUsage[];
  daily: DailyUsage[];
  totals: UsageTotals;
  /** Hivex Harness turn budget: { allowed, used, cap, capped, remaining } (cap null = uncapped). */
  turns: TurnBudget;
}

export interface TurnBudget {
  allowed: boolean;
  used: number;
  cap: number | null;
  capped: boolean;
  remaining: number | null;
}

/** POST /api/portal/turns — { success: true, ...budget }. */
export interface TurnsReportResponse extends TurnBudget {
  success: boolean;
}

/** GET /api/portal/entitlement. */
export interface EntitlementResponse {
  token: string;
  entitlement: {
    account: string | null;
    tier: string;
    turn_cap_monthly: number | null;
    provider_locked: boolean;
    issued_at: number;
    exp: number;
  };
}

/** POST /api/keys — 201 with the full key value returned exactly once. */
export interface AdminCreateKeyResponse {
  key: string;
  name: string;
  id: string;
  machineId: string;
}

/** DELETE /api/keys/{id}. */
export interface AdminDeleteKeyResponse {
  message: string;
}

export class GatewayHTTPError extends Error {
  override readonly name = "GatewayHTTPError";
  constructor(
    readonly status: number,
    message: string,
    readonly url: string,
    readonly bodyText: string = "",
  ) {
    super(message);
  }
}

// ---------------------------------------------------------------------------
// Fetch plumbing
// ---------------------------------------------------------------------------

function joinURL(base: string, path: string): string {
  return `${base}${path.startsWith("/") ? path : `/${path}`}`;
}

async function readError(response: Response, url: string): Promise<GatewayHTTPError> {
  const text = await response.text().catch(() => "");
  let message = text || `HTTP ${response.status}`;
  try {
    const parsed = JSON.parse(text) as { error?: unknown; message?: unknown };
    if (typeof parsed.error === "string") message = parsed.error;
    else if (typeof parsed.error === "object" && parsed.error !== null) {
      const err = parsed.error as { message?: unknown };
      if (typeof err.message === "string") message = err.message;
    } else if (typeof parsed.message === "string") message = parsed.message;
  } catch {
    // Non-JSON error body.
  }
  return new GatewayHTTPError(response.status, message, url, text);
}

interface FetchOptions {
  method: "GET" | "POST" | "DELETE";
  path: string;
  headers?: Record<string, string>;
  body?: unknown;
}

async function fetchJSON<T>(env: HiveEnv, opts: FetchOptions): Promise<T> {
  const url = joinURL(env.baseURL, opts.path);
  let response: Response;
  try {
    response = await fetch(url, {
      method: opts.method,
      headers: {
        ...(opts.body !== undefined ? { "Content-Type": "application/json" } : {}),
        ...opts.headers,
      },
      body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    });
  } catch (cause) {
    const detail = cause instanceof Error ? `: ${cause.message}` : "";
    throw new GatewayHTTPError(0, `Could not reach the HiveAPI Gateway at ${env.baseURL}${detail}`, url);
  }
  if (!response.ok) throw await readError(response, url);
  return (await response.json()) as T;
}

function portalSessionHeaders(env: HiveEnv): Record<string, string> {
  return env.portalCookie ? { Cookie: `auth_token=${env.portalCookie}` } : {};
}

function accountKeyHeaders(env: HiveEnv): Record<string, string> {
  return env.apiKey ? { Authorization: `Bearer ${env.apiKey}` } : {};
}

/**
 * Admin auth for /api/keys. The gateway accepts the dashboard JWT via the
 * `auth_token` cookie, or the local CLI token via `x-9r-cli-token`
 * (9router/src/dashboardGuard.js). HIVEAPI_ADMIN_TOKEN is sent on all three
 * channels so whichever credential form the operator provisioned works.
 */
function adminHeaders(env: HiveEnv): Record<string, string> {
  if (!env.adminToken) return {};
  return {
    Authorization: `Bearer ${env.adminToken}`,
    "x-9r-cli-token": env.adminToken,
    Cookie: `auth_token=${env.adminToken}`,
  };
}

// ---------------------------------------------------------------------------
// Portal surfaces
// ---------------------------------------------------------------------------

/** GET /api/portal/usage — per-key monthly usage + harness turn budget. Requires a portal session cookie. */
export function fetchUsage(env: HiveEnv): Promise<PortalUsageResponse> {
  return fetchJSON(env, {
    method: "GET",
    path: "/api/portal/usage",
    headers: portalSessionHeaders(env),
  });
}

/** GET /api/portal/entitlement — the signed entitlement token for the portal account. */
export function fetchEntitlement(env: HiveEnv): Promise<EntitlementResponse> {
  return fetchJSON(env, {
    method: "GET",
    path: "/api/portal/entitlement",
    headers: portalSessionHeaders(env),
  });
}

/** POST /api/portal/turns — report harness turns (auth: account gateway key or portal session). */
export function reportTurns(env: HiveEnv, turns: number): Promise<TurnsReportResponse> {
  return fetchJSON<TurnsReportResponse>(env, {
    method: "POST",
    path: "/api/portal/turns",
    headers: accountKeyHeaders(env),
    body: { turns },
  });
}

// ---------------------------------------------------------------------------
// Admin key store
// ---------------------------------------------------------------------------

/** POST /api/keys — create a gateway API key ({name} → 201 {key, name, id, machineId}). */
export function adminCreateKey(env: HiveEnv, name: string): Promise<AdminCreateKeyResponse> {
  return fetchJSON(env, {
    method: "POST",
    path: "/api/keys",
    headers: adminHeaders(env),
    body: { name },
  });
}

/** DELETE /api/keys/{id} — delete (revoke) a gateway API key. */
export function adminRevokeKey(env: HiveEnv, id: string): Promise<AdminDeleteKeyResponse> {
  return fetchJSON(env, {
    method: "DELETE",
    path: `/api/keys/${encodeURIComponent(id)}`,
    headers: adminHeaders(env),
  });
}
