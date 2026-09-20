import { afterAll, beforeAll, describe, expect, it } from "bun:test";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { InMemoryTransport } from "@modelcontextprotocol/sdk/inMemory.js";

import type { HiveEnv } from "../src/env.ts";
import { ADMIN_GATING_MESSAGE, PORTAL_COOKIE_HINT } from "../src/tools.ts";
import {
  hiveapiKeyCreate,
  hiveapiKeyCreateInput,
  hiveapiKeyRevoke,
  hiveapiKeyRevokeInput,
  hiveapiEntitlement,
  hiveapiModels,
  hiveapiTurnsReportInput,
  hiveapiUsage,
} from "../src/tools.ts";
import { createServer } from "../src/server.ts";
import { startMockGateway, MOCK_ENTITLEMENT_SECRET, type MockGateway } from "./mockGateway.ts";

let mock: MockGateway;
let env: HiveEnv;

function parseResult(result: { content: Array<{ type: string; text: string }> }) {
  return JSON.parse(result.content[0]?.text ?? "{}");
}

beforeAll(() => {
  mock = startMockGateway();
  env = {
    baseURL: mock.url,
    apiKey: "sk-a1b2c3d4e5f6a7b8-k3y777-cafe1234",
    portalCookie: "portal-session-jwt-abc123",
    entitlementSecret: MOCK_ENTITLEMENT_SECRET,
    adminToken: null,
  };
});

afterAll(() => {
  mock.close();
});

describe("tool input schemas (zod)", () => {
  it("hiveapi_key_create requires a non-empty name", () => {
    expect(hiveapiKeyCreateInput.name.parse("harness-key")).toBe("harness-key");
    expect(() => hiveapiKeyCreateInput.name.parse("")).toThrow();
    expect(() => hiveapiKeyCreateInput.name.parse(undefined)).toThrow();
    expect(() => hiveapiKeyCreateInput.name.parse(42)).toThrow();
  });

  it("hiveapi_key_revoke requires a non-empty id", () => {
    expect(hiveapiKeyRevokeInput.id.parse("key-uuid-001")).toBe("key-uuid-001");
    expect(() => hiveapiKeyRevokeInput.id.parse("")).toThrow();
    expect(() => hiveapiKeyRevokeInput.id.parse(undefined)).toThrow();
  });

  it("hiveapi_turns_report accepts positive integers and defaults to 1", () => {
    const schema = hiveapiTurnsReportInput.turns;
    expect(schema.parse(1)).toBe(1);
    expect(schema.parse(5)).toBe(5);
    expect(schema.parse(undefined)).toBe(1);
    expect(() => schema.parse(0)).toThrow();
    expect(() => schema.parse(-2)).toThrow();
    expect(() => schema.parse("three")).toThrow();
    expect(() => schema.parse(1.5)).toThrow();
  });
});

describe("admin-gated tools", () => {
  it("hiveapi_key_create answers 'admin gating not configured' without HIVEAPI_ADMIN_TOKEN", async () => {
    const result = await hiveapiKeyCreate({ name: "ops-key" }, env);
    // A plain answer, not an error — the MCP session keeps working.
    expect(result.isError).toBeFalsy();
    expect(result.content[0]?.text).toBe(ADMIN_GATING_MESSAGE);
    expect(mock.state.createdKeyNames).toEqual([]);
  });

  it("hiveapi_key_revoke answers 'admin gating not configured' without HIVEAPI_ADMIN_TOKEN", async () => {
    const result = await hiveapiKeyRevoke({ id: "key-uuid-001" }, env);
    expect(result.isError).toBeFalsy();
    expect(result.content[0]?.text).toBe(ADMIN_GATING_MESSAGE);
    expect(mock.state.revokedKeyIds).toEqual([]);
  });

  it("hiveapi_key_create creates a key through the real POST /api/keys route when gated open", async () => {
    const admin: HiveEnv = { ...env, adminToken: "dashboard-jwt-or-cli-token" };
    const result = await hiveapiKeyCreate({ name: "ops-key" }, admin);
    const body = parseResult(result);
    expect(body.created).toBe(true);
    expect(body.key).toBe("sk-a1b2c3d4e5f6a7b8-k3y123-cafe1234");
    expect(body.id).toBe("key-uuid-001");
    expect(body.machineId).toBe("a1b2c3d4e5f6a7b8");
    expect(mock.state.createdKeyNames).toEqual(["ops-key"]);
    // The admin token is sent on every auth channel the gateway accepts.
    expect(mock.state.keysAuth.bearer).toBe("Bearer dashboard-jwt-or-cli-token");
    expect(mock.state.keysAuth.cliToken).toBe("dashboard-jwt-or-cli-token");
    expect(mock.state.keysAuth.cookie).toBe("auth_token=dashboard-jwt-or-cli-token");
  });

  it("hiveapi_key_revoke deletes through the real DELETE /api/keys/{id} route when gated open", async () => {
    const admin: HiveEnv = { ...env, adminToken: "dashboard-jwt-or-cli-token" };
    const result = await hiveapiKeyRevoke({ id: "key-uuid-001" }, admin);
    const body = parseResult(result);
    expect(body.revoked).toBe(true);
    expect(body.message).toBe("Key deleted successfully");
    expect(mock.state.revokedKeyIds).toEqual(["key-uuid-001"]);
  });
});

describe("hiveapi_models", () => {
  it("lists models via @hiveapi/client against GET /v1/models", async () => {
    const result = await hiveapiModels({}, env);
    const body = parseResult(result);
    expect(body.count).toBe(2);
    expect(body.models[0]).toEqual({ id: "openai/gpt-5", owned_by: "openai", context_length: 400000 });
    expect(body.models[1]).toEqual({ id: "cc/claude-opus-4-7", owned_by: "cc" });
  });

  it("surfaces gateway HTTP errors as classified error results", async () => {
    const broken: HiveEnv = { ...env, baseURL: "http://127.0.0.1:1" };
    const result = await hiveapiModels({}, broken);
    expect(result.isError).toBe(true);
    expect(result.content[0]?.text).toContain("HiveAPI error (unreachable)");
  });
});

describe("hiveapi_usage", () => {
  it("hints instead of calling when HIVEAPI_PORTAL_COOKIE is not set", async () => {
    const result = await hiveapiUsage({}, { ...env, portalCookie: null });
    expect(result.isError).toBe(true);
    expect(result.content[0]?.text).toBe(PORTAL_COOKIE_HINT);
  });

  it("returns the real monthly usage shape with a portal session cookie", async () => {
    const result = await hiveapiUsage({}, env);
    const body = parseResult(result);
    expect(body.monthStart).toBe("2026-09-01T00:00:00.000Z");
    expect(body.tier).toBe("monthly");
    expect(body.totals).toEqual({ requests: 3, promptTokens: 10, completionTokens: 5, totalTokens: 15 });
    expect(body.keys[0].maskedKey).toBe("sk-a1b2c3***");
    expect(body.turns).toEqual({ allowed: true, used: 5, cap: null, capped: false, remaining: null });
    expect(mock.state.usageAuth).toBe("auth_token=portal-session-jwt-abc123");
  });
});

describe("hiveapi_entitlement", () => {
  it("fetches the token, verifies the HMAC and returns the entitlement payload", async () => {
    const result = await hiveapiEntitlement({}, env);
    const body = parseResult(result);
    expect(body.verified).toBe(true);
    expect(body.verification).toBe("HMAC-SHA256 signature valid and not expired");
    expect(body.entitlement.account).toBe("acct-b33fcafe");
    expect(body.entitlement.tier).toBe("monthly");
    expect(body.entitlement.provider_locked).toBe(true);
  });

  it("reports a mismatched signature as an error result", async () => {
    const wrongSecret: HiveEnv = { ...env, entitlementSecret: "not-the-gateway-secret" };
    const result = await hiveapiEntitlement({}, wrongSecret);
    expect(result.isError).toBe(true);
    const body = parseResult(result);
    expect(body.verified).toBe(false);
    expect(body.verification).toBe("rejected (bad_signature)");
  });

  it("explains when verification is not configured", async () => {
    const noSecret: HiveEnv = { ...env, entitlementSecret: null };
    const result = await hiveapiEntitlement({}, noSecret);
    expect(result.isError).toBeFalsy();
    const body = parseResult(result);
    expect(body.verified).toBe(false);
    expect(body.verification).toContain("HIVEAPI_ENTITLEMENT_SECRET");
  });
});

describe("MCP server over InMemoryTransport", () => {
  it("lists all six tools with schemas", async () => {
    const server = createServer(env);
    const client = new Client({ name: "test-client", version: "0.0.0" });
    const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
    // Both sides must be connected before the first request — in-memory
    // messages written before the peer starts are dropped.
    await Promise.all([client.connect(clientTransport), server.connect(serverTransport)]);

    const { tools } = await client.listTools();
    expect(tools.map((t) => t.name)).toEqual([
      "hiveapi_models",
      "hiveapi_usage",
      "hiveapi_entitlement",
      "hiveapi_key_create",
      "hiveapi_key_revoke",
      "hiveapi_turns_report",
    ]);
    const keyCreate = tools.find((t) => t.name === "hiveapi_key_create");
    expect(keyCreate?.description).toContain("HIVEAPI_ADMIN_TOKEN");

    await client.close();
    await server.close();
  });

  it("serves a tool call end-to-end and validates input schemas", async () => {
    const server = createServer(env);
    const client = new Client({ name: "test-client", version: "0.0.0" });
    const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
    await Promise.all([client.connect(clientTransport), server.connect(serverTransport)]);

    // Happy path through the full MCP layer.
    const ok = await client.callTool({ name: "hiveapi_models", arguments: {} });
    const okText = (ok.content as Array<{ type: string; text: string }>)[0]?.text ?? "";
    expect(JSON.parse(okText).count).toBe(2);

    // Invalid input (missing required name) must be rejected by schema
    // validation — either a protocol error or an isError result, never a
    // silent success.
    let rejected = false;
    try {
      const bad = await client.callTool({ name: "hiveapi_key_create", arguments: {} });
      rejected = bad.isError === true;
    } catch {
      rejected = true;
    }
    expect(rejected).toBe(true);

    // Admin gate through the MCP layer: graceful answer, no error.
    const gated = await client.callTool({
      name: "hiveapi_key_create",
      arguments: { name: "nope" },
    });
    const gatedText = (gated.content as Array<{ type: string; text: string }>)[0]?.text ?? "";
    expect(gatedText).toBe(ADMIN_GATING_MESSAGE);

    await client.close();
    await server.close();
  });
});
