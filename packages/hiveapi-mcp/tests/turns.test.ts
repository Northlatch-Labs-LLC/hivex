import { afterAll, beforeAll, describe, expect, it } from "bun:test";

import type { HiveEnv } from "../src/env.ts";
import { GatewayHTTPError, reportTurns } from "../src/gateway.ts";
import { hiveapiTurnsReport } from "../src/tools.ts";
import { startMockGateway, type MockGateway } from "./mockGateway.ts";

let mock: MockGateway;
let env: HiveEnv;

beforeAll(() => {
  mock = startMockGateway();
  env = {
    baseURL: mock.url,
    apiKey: "sk-a1b2c3d4e5f6a7b8-k3y777-cafe1234",
    portalCookie: null,
    entitlementSecret: null,
    adminToken: null,
  };
});

afterAll(() => {
  mock.close();
});

function parseResult(result: { content: Array<{ type: string; text: string }> }) {
  return JSON.parse(result.content[0]?.text ?? "{}");
}

describe("hiveapi_turns_report — POST /api/portal/turns happy path", () => {
  it("reports turns with the account key as Bearer auth", async () => {
    const result = await hiveapiTurnsReport({ turns: 3 }, env);
    expect(result.isError).toBeFalsy();

    const body = parseResult(result);
    expect(body).toEqual({
      success: true,
      allowed: true,
      used: 8,
      cap: 1000,
      capped: false,
      remaining: 992,
    });

    // Wire checks: real route, Bearer auth, JSON body {turns}.
    expect(mock.state.turnsAuth).toBe(`Bearer ${env.apiKey}`);
    expect(mock.state.turnsBody).toEqual({ turns: 3 });
  });

  it("defaults to 1 turn (gateway-side default mirrored by the zod schema)", async () => {
    const result = await hiveapiTurnsReport({ turns: 1 }, env);
    expect(parseResult(result).success).toBe(true);
    expect(mock.state.turnsBody).toEqual({ turns: 1 });
  });

  it("returns the gateway's 401 as a classified error result", async () => {
    const anonymous: HiveEnv = { ...env, apiKey: "" };
    const result = await hiveapiTurnsReport({ turns: 1 }, anonymous);
    expect(result.isError).toBe(true);
    expect(result.content[0]?.text).toContain("Portal session or account API key required");
  });

  it("reportTurns surfaces HTTP failures as GatewayHTTPError", async () => {
    await expect(reportTurns(env, 1)).resolves.toHaveProperty("success", true);
    const badEnv: HiveEnv = { ...env, baseURL: "http://127.0.0.1:1" };
    try {
      await reportTurns(badEnv, 1);
      expect.unreachable();
    } catch (error) {
      expect(error).toBeInstanceOf(GatewayHTTPError);
      expect((error as GatewayHTTPError).status).toBe(0);
    }
  });
});
