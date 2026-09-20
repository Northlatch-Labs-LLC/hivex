/**
 * Entitlement token signing/verification — an exact mirror of the gateway's
 * 9router/src/lib/portal/entitlements.js:
 *
 *   token = base64url(JSON payload) + "." + base64url(HMAC-SHA256(payloadB64url, key))
 *
 * The payload carries { account, tier, turn_cap_monthly, provider_locked,
 * issued_at, exp } with exp in SECONDS. Verification is constant-time and
 * rejects expired tokens (exp * 1000 <= now).
 */

import {
  createHmac,
  timingSafeEqual,
} from "node:crypto";

export interface EntitlementPayload {
  account: string | null;
  tier: "free" | "monthly" | "business" | string;
  /** Monthly harness turn cap; null means uncapped. */
  turn_cap_monthly: number | null;
  /** True when the tier is locked to this gateway (monthly tier). */
  provider_locked: boolean;
  issued_at: number;
  exp: number;
  [key: string]: unknown;
}

function b64url(input: Uint8Array | string): string {
  return Buffer.from(input).toString("base64url");
}

function hmac(payloadB64url: string, key: string): Buffer {
  return createHmac("sha256", key).update(payloadB64url).digest();
}

/** Sign a payload exactly the way the gateway does (gateway entitlements.js signEntitlementToken). */
export function signEntitlementToken(payload: EntitlementPayload, key: string): string {
  const payloadB64url = b64url(JSON.stringify(payload));
  const sig = hmac(payloadB64url, key);
  return `${payloadB64url}.${b64url(sig)}`;
}

export type VerifyFailure =
  | "malformed"
  | "bad_signature"
  | "bad_payload"
  | "expired";

export interface VerifySuccess {
  ok: true;
  payload: EntitlementPayload;
}

export interface VerifyError {
  ok: false;
  reason: VerifyFailure;
}

export type VerifyResult = VerifySuccess | VerifyError;

/** Verify a token with the shared secret (gateway entitlements.js verifyEntitlementToken). */
export function verifyEntitlementToken(
  token: string,
  key: string,
  nowMs: number = Date.now(),
): VerifyResult {
  const parts = token.split(".");
  if (parts.length !== 2 || !parts[0] || !parts[1]) return { ok: false, reason: "malformed" };
  const [payloadB64url, sigB64url] = parts;

  let provided: Buffer;
  try {
    provided = Buffer.from(sigB64url, "base64url");
  } catch {
    return { ok: false, reason: "malformed" };
  }
  const expected = hmac(payloadB64url, key);
  if (provided.length !== expected.length) return { ok: false, reason: "bad_signature" };
  if (!timingSafeEqual(provided, expected)) return { ok: false, reason: "bad_signature" };

  let payload: EntitlementPayload;
  try {
    payload = JSON.parse(Buffer.from(payloadB64url, "base64url").toString("utf8")) as EntitlementPayload;
  } catch {
    return { ok: false, reason: "bad_payload" };
  }
  if (!payload || typeof payload !== "object") return { ok: false, reason: "bad_payload" };

  const exp = Number(payload.exp);
  if (!Number.isFinite(exp) || nowMs > exp * 1000) return { ok: false, reason: "expired" };

  return { ok: true, payload };
}
