import { describe, expect, it } from "bun:test";

import { HiveAPIClient } from "../src/client.ts";
import { HiveAPIError, classifyStatus, isHiveAPIError, type HiveAPIErrorKind } from "../src/errors.ts";
import { gatewayErrorBody } from "./fixtures.ts";

function jsonResponse(status: number, body: Record<string, unknown>, headers: Record<string, string> = {}) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", ...headers },
  });
}

function clientWithStatus(status: number, message = `HTTP ${status}`, headers: Record<string, string> = {}) {
  return new HiveAPIClient({
    baseURL: "https://gw.example",
    apiKey: "sk-fixedmachine-abc123-a1b2c3d4",
    fetchImpl: () =>
      Promise.resolve(jsonResponse(status, gatewayErrorBody(status, message), headers)),
  });
}

async function expectKind(status: number, kind: HiveAPIErrorKind, headers: Record<string, string> = {}) {
  const client = clientWithStatus(status, `Error ${status}`, headers);
  try {
    await client.models.list();
    expect.unreachable();
  } catch (error) {
    expect(error).toBeInstanceOf(HiveAPIError);
    expect((error as HiveAPIError).kind).toBe(kind);
    expect((error as HiveAPIError).status).toBe(status);
  }
}

describe("error classification from status codes", () => {
  it("maps the raw status space", () => {
    expect(classifyStatus(400)).toBe("invalid_request");
    expect(classifyStatus(404)).toBe("invalid_request");
    expect(classifyStatus(401)).toBe("unauthorized");
    expect(classifyStatus(402)).toBe("unauthorized");
    expect(classifyStatus(403)).toBe("unauthorized");
    expect(classifyStatus(429)).toBe("rate_limited");
    // The gateway answers 503 (with Retry-After) when all upstream accounts
    // are exhausted — 9router/open-sse/utils/error.js unavailableResponse().
    expect(classifyStatus(503)).toBe("rate_limited");
    expect(classifyStatus(500)).toBe("server_error");
    expect(classifyStatus(502)).toBe("server_error");
    expect(classifyStatus(504)).toBe("server_error");
  });

  it("classifies 401 as unauthorized with the gateway error body", async () => {
    await expectKind(401, "unauthorized");
    const client = clientWithStatus(401, "Invalid API key provided");
    try {
      await client.models.list();
    } catch (error) {
      const hiveError = error as HiveAPIError;
      expect(hiveError.message).toBe("Invalid API key provided");
      expect(hiveError.type).toBe("authentication_error");
      expect(hiveError.code).toBe("invalid_api_key");
      expect(isHiveAPIError(error)).toBe(true);
    }
  });

  it("classifies 402/403 as unauthorized", async () => {
    await expectKind(402, "unauthorized");
    await expectKind(403, "unauthorized");
  });

  it("classifies 429 as rate_limited and parses Retry-After", async () => {
    const client = clientWithStatus(429, "Rate limit exceeded", { "Retry-After": "30" });
    try {
      await client.models.list();
      expect.unreachable();
    } catch (error) {
      const hiveError = error as HiveAPIError;
      expect(hiveError.kind).toBe("rate_limited");
      expect(hiveError.retryAfterSeconds).toBe(30);
      expect(hiveError.type).toBe("rate_limit_error");
    }
  });

  it("classifies the gateway's 503 upstream-exhausted answer as rate_limited", async () => {
    const client = clientWithStatus(
      503,
      "[openai/gpt-5] Unavailable (reset after 30s)",
      { "Retry-After": "30" },
    );
    try {
      await client.chat.completions.create({ model: "openai/gpt-5", messages: [] });
      expect.unreachable();
    } catch (error) {
      const hiveError = error as HiveAPIError;
      expect(hiveError.kind).toBe("rate_limited");
      expect(hiveError.retryAfterSeconds).toBe(30);
    }
  });

  it("classifies 5xx as server_error", async () => {
    await expectKind(500, "server_error");
    await expectKind(502, "server_error");
    await expectKind(504, "server_error");
  });

  it("classifies other 4xx as invalid_request", async () => {
    await expectKind(400, "invalid_request");
    await expectKind(404, "invalid_request");
  });
});

describe("unreachable and abort handling", () => {
  it("classifies fetch failures as unreachable and keeps the cause", async () => {
    const client = new HiveAPIClient({
      baseURL: "https://gw.example",
      apiKey: "sk-test",
      fetchImpl: () => Promise.reject(new TypeError("fetch failed")),
    });
    try {
      await client.models.list();
      expect.unreachable();
    } catch (error) {
      const hiveError = error as HiveAPIError;
      expect(hiveError.kind).toBe("unreachable");
      expect(hiveError.status).toBeNull();
      expect(hiveError.isUnreachable).toBe(true);
      expect(hiveError.message).toContain("Could not reach the HiveAPI Gateway");
      expect((hiveError.cause as Error).message).toBe("fetch failed");
    }
  });

  it("rethrows AbortError untouched", async () => {
    const abort = new DOMException("The operation was aborted", "AbortError");
    const client = new HiveAPIClient({
      baseURL: "https://gw.example",
      apiKey: "sk-test",
      fetchImpl: () => Promise.reject(abort),
    });
    try {
      await client.models.list();
      expect.unreachable();
    } catch (error) {
      expect((error as DOMException).name).toBe("AbortError");
      expect(error).not.toBeInstanceOf(HiveAPIError);
    }
  });

  it("tolerates non-JSON error bodies", async () => {
    const client = new HiveAPIClient({
      baseURL: "https://gw.example",
      apiKey: "sk-test",
      fetchImpl: () => Promise.resolve(new Response("upstream exploded", { status: 502 })),
    });
    try {
      await client.models.list();
      expect.unreachable();
    } catch (error) {
      const hiveError = error as HiveAPIError;
      expect(hiveError.kind).toBe("server_error");
      expect(hiveError.message).toBe("upstream exploded");
    }
  });
});
