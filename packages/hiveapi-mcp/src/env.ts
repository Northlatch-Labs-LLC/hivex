/**
 * Environment contract for the @hiveapi/mcp server.
 *
 * | Variable                     | Required | Meaning |
 * |------------------------------|----------|---------|
 * | HIVEAPI_BASE_URL             | yes      | Gateway base URL (e.g. http://127.0.0.1:8317). |
 * | HIVEAPI_API_KEY              | yes*     | Gateway API key (Bearer). *Only optional for a local gateway with requireApiKey=false. |
 * | HIVEAPI_PORTAL_COOKIE        | no       | Portal session `auth_token` cookie value — enables entitlement/usage fetches (browser session auth). |
 * | HIVEAPI_ENTITLEMENT_SECRET   | no       | HMAC secret shared with the gateway (ENTITLEMENT_SIGNING_KEY) — enables entitlement verification. |
 * | HIVEAPI_ADMIN_TOKEN          | no       | Admin credential the gateway accepts for /api/keys. When unset, key tools answer "admin gating not configured". |
 */

export interface HiveEnv {
  /** Raw gateway base URL (no /v1 appended — portal/admin routes live at the root). */
  readonly baseURL: string;
  readonly apiKey: string;
  readonly portalCookie: string | null;
  readonly entitlementSecret: string | null;
  readonly adminToken: string | null;
}

export class MissingEnvError extends Error {
  override readonly name = "MissingEnvError";
  constructor(readonly keys: string[]) {
    super(`Missing required environment variable(s): ${keys.join(", ")}`);
  }
}

export function loadEnv(source: Record<string, string | undefined> = process.env): HiveEnv {
  const missing: string[] = [];
  const baseURL = source["HIVEAPI_BASE_URL"] ?? "";
  if (!baseURL) missing.push("HIVEAPI_BASE_URL");
  const apiKey = source["HIVEAPI_API_KEY"] ?? "";
  if (!apiKey) missing.push("HIVEAPI_API_KEY");
  if (missing.length > 0) throw new MissingEnvError(missing);

  return {
    baseURL: baseURL.replace(/\/+$/, ""),
    apiKey,
    portalCookie: source["HIVEAPI_PORTAL_COOKIE"] || null,
    entitlementSecret: source["HIVEAPI_ENTITLEMENT_SECRET"] || null,
    adminToken: source["HIVEAPI_ADMIN_TOKEN"] || null,
  };
}
