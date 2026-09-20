import { describe, expect, it } from "bun:test";

import {
  signEntitlementToken,
  verifyEntitlementToken,
  type EntitlementPayload,
} from "../src/entitlement.ts";
import { createHmac } from "node:crypto";

const SECRET = "gateway-signing-key-7f3a9c";
const OTHER_SECRET = "attacker-guessing-key";

const NOW_MS = 1758326400000;
const NOW_SEC = Math.floor(NOW_MS / 1000);

function samplePayload(exp: number = NOW_SEC + 600): EntitlementPayload {
  return {
    account: "acct-b33fcafe",
    tier: "monthly",
    turn_cap_monthly: null,
    provider_locked: true,
    issued_at: NOW_SEC,
    exp,
  };
}

/** Independent re-implementation of the gateway's entitlements.js for cross-checking. */
function gatewaySign(payload: EntitlementPayload, key: string): string {
  const payloadB64url = Buffer.from(JSON.stringify(payload)).toString("base64url");
  const sig = createHmac("sha256", key).update(payloadB64url).digest();
  return `${payloadB64url}.${Buffer.from(sig).toString("base64url")}`;
}

describe("entitlement token — gateway algorithm parity", () => {
  it("signs byte-identically to an independent gateway re-implementation", () => {
    const payload = samplePayload();
    expect(signEntitlementToken(payload, SECRET)).toBe(gatewaySign(payload, SECRET));
  });

  it("uses base64url segments with no padding", () => {
    const token = signEntitlementToken(samplePayload(), SECRET);
    expect(token).toMatch(/^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/);
    expect(token).not.toContain("=");
  });
});

describe("verifyEntitlementToken — round-trip", () => {
  it("accepts a freshly signed token and returns the payload", () => {
    const token = signEntitlementToken(samplePayload(), SECRET);
    const result = verifyEntitlementToken(token, SECRET, NOW_MS);
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.payload.account).toBe("acct-b33fcafe");
      expect(result.payload.tier).toBe("monthly");
      expect(result.payload.turn_cap_monthly).toBeNull();
      expect(result.payload.provider_locked).toBe(true);
      expect(result.payload.exp).toBe(NOW_SEC + 600);
    }
  });

  it("accepts a token signed by the independent gateway implementation", () => {
    const token = gatewaySign(samplePayload(), SECRET);
    const result = verifyEntitlementToken(token, SECRET, NOW_MS);
    expect(result.ok).toBe(true);
  });

  it("accepts a free-tier token with a numeric cap", () => {
    const payload: EntitlementPayload = {
      account: "acct-free",
      tier: "free",
      turn_cap_monthly: 1000,
      provider_locked: false,
      issued_at: NOW_SEC,
      exp: NOW_SEC + 600,
    };
    const result = verifyEntitlementToken(signEntitlementToken(payload, SECRET), SECRET, NOW_MS);
    expect(result.ok && result.payload.turn_cap_monthly).toBe(1000);
  });
});

describe("verifyEntitlementToken — rejections", () => {
  it("rejects a token signed with a different key (bad_signature)", () => {
    const token = signEntitlementToken(samplePayload(), OTHER_SECRET);
    const result = verifyEntitlementToken(token, SECRET, NOW_MS);
    expect(result).toEqual({ ok: false, reason: "bad_signature" });
  });

  it("rejects a tampered payload (bad_signature)", () => {
    const token = signEntitlementToken(samplePayload(), SECRET);
    const [payloadB64url, sigB64url] = token.split(".");
    const tampered = {
      account: "acct-b33fcafe",
      tier: "business",
      turn_cap_monthly: null,
      provider_locked: false,
      issued_at: NOW_SEC,
      exp: NOW_SEC + 600,
    };
    const tamperedPayload = Buffer.from(JSON.stringify(tampered)).toString("base64url");
    const result = verifyEntitlementToken(`${tamperedPayload}.${sigB64url}`, SECRET, NOW_MS);
    expect(result).toEqual({ ok: false, reason: "bad_signature" });
    void payloadB64url;
  });

  it("rejects an expired token (expired)", () => {
    const token = signEntitlementToken(samplePayload(NOW_SEC - 1), SECRET);
    const result = verifyEntitlementToken(token, SECRET, NOW_MS);
    expect(result).toEqual({ ok: false, reason: "expired" });
  });

  it("rejects malformed tokens (malformed)", () => {
    expect(verifyEntitlementToken("", SECRET, NOW_MS)).toEqual({ ok: false, reason: "malformed" });
    expect(verifyEntitlementToken("no-separator", SECRET, NOW_MS)).toEqual({
      ok: false,
      reason: "malformed",
    });
    expect(verifyEntitlementToken("a.b.c", SECRET, NOW_MS)).toEqual({
      ok: false,
      reason: "malformed",
    });
  });

  it("rejects a non-JSON payload (bad_payload)", () => {
    const payloadB64url = Buffer.from("not json at all").toString("base64url");
    const sig = createHmac("sha256", SECRET).update(payloadB64url).digest();
    const token = `${payloadB64url}.${Buffer.from(sig).toString("base64url")}`;
    const result = verifyEntitlementToken(token, SECRET, NOW_MS);
    expect(result).toEqual({ ok: false, reason: "bad_payload" });
  });
});
