/**
 * Bun.serve mock of the gateway surfaces the MCP tools actually hit,
 * returning the REAL response shapes (portal usage/entitlement/turns,
 * admin key store, /v1/models). Lets the tests exercise full HTTP paths.
 */

import { signEntitlementToken, type EntitlementPayload } from "../src/entitlement.ts";

export const MOCK_ENTITLEMENT_SECRET = "mock-entitlement-secret-0123456789abcdef";

export interface MockGatewayState {
  /** Authorization header captured on the last /api/portal/turns call. */
  turnsAuth: string | null;
  /** Parsed body captured on the last /api/portal/turns call. */
  turnsBody: unknown;
  /** Cookie header captured on the last /api/portal/usage call. */
  usageAuth: string | null;
  /** Authorization/x-9r-cli-token/Cookie captured on the last /api/keys call. */
  keysAuth: { bearer: string | null; cliToken: string | null; cookie: string | null };
  createdKeyNames: string[];
  revokedKeyIds: string[];
  /** Secret the mock uses to sign entitlement tokens (change per test). */
  entitlementSigningSecret: string;
}

export interface MockGateway {
  url: string;
  state: MockGatewayState;
  close(): void;
}

export function startMockGateway(overrides?: Partial<MockGatewayState>): MockGateway {
  const state: MockGatewayState = {
    turnsAuth: null,
    turnsBody: null,
    usageAuth: null,
    keysAuth: { bearer: null, cliToken: null, cookie: null },
    createdKeyNames: [],
    revokedKeyIds: [],
    entitlementSigningSecret: MOCK_ENTITLEMENT_SECRET,
    ...overrides,
  };

  const usageBody = {
    monthStart: "2026-09-01T00:00:00.000Z",
    tier: "monthly",
    keys: [
      {
        id: "b7e6c9a0-1111-2222-3333-444455556666",
        name: "harness",
        maskedKey: "sk-a1b2c3***",
        isActive: true,
        requests: 3,
        promptTokens: 10,
        completionTokens: 5,
        totalTokens: 15,
      },
    ],
    daily: [
      {
        date: "2026-09-18",
        requests: 3,
        promptTokens: 10,
        completionTokens: 5,
        totalTokens: 15,
      },
    ],
    totals: { requests: 3, promptTokens: 10, completionTokens: 5, totalTokens: 15 },
    turns: { allowed: true, used: 5, cap: null, capped: false, remaining: null },
  };

  const server = Bun.serve({
    port: 0,
    idleTimeout: 5,
    async fetch(request) {
      const url = new URL(request.url);

      // GET /v1/models — the client auto-appends /v1.
      if (url.pathname === "/v1/models" && request.method === "GET") {
        return Response.json({
          object: "list",
          data: [
            { id: "openai/gpt-5", object: "model", owned_by: "openai", context_length: 400000 },
            { id: "cc/claude-opus-4-7", object: "model", owned_by: "cc" },
          ],
        });
      }

      // POST /api/portal/turns — Bearer key OR portal session (route accepts both).
      if (url.pathname === "/api/portal/turns" && request.method === "POST") {
        state.turnsAuth = request.headers.get("authorization");
        state.turnsBody = await request.json().catch(() => null);
        const body = state.turnsBody as { turns?: number } | null;
        if (!request.headers.get("authorization") && !request.headers.get("cookie")) {
          return Response.json({ error: "Portal session or account API key required" }, { status: 401 });
        }
        const turns = typeof body?.turns === "number" ? body.turns : 1;
        const used = 5 + turns;
        const cap = 1000;
        return Response.json({
          success: true,
          allowed: used < cap,
          used,
          cap,
          capped: used >= cap,
          remaining: cap - used,
        });
      }

      // GET /api/portal/usage — portal session cookie only.
      if (url.pathname === "/api/portal/usage" && request.method === "GET") {
        state.usageAuth = request.headers.get("cookie");
        if (!request.headers.get("cookie")?.includes("auth_token=")) {
          return Response.json({ error: "Portal login required" }, { status: 401 });
        }
        return Response.json(usageBody);
      }

      // GET /api/portal/entitlement — portal session cookie only.
      if (url.pathname === "/api/portal/entitlement" && request.method === "GET") {
        if (!request.headers.get("cookie")?.includes("auth_token=")) {
          return Response.json({ error: "Portal login required" }, { status: 401 });
        }
        const nowSec = Math.floor(Date.now() / 1000);
        const payload: EntitlementPayload = {
          account: "acct-b33fcafe",
          tier: "monthly",
          turn_cap_monthly: null,
          provider_locked: true,
          issued_at: nowSec,
          exp: nowSec + 600,
        };
        return Response.json({ token: signEntitlementToken(payload, state.entitlementSigningSecret), entitlement: payload });
      }

      // POST /api/keys — admin key store.
      if (url.pathname === "/api/keys" && request.method === "POST") {
        state.keysAuth = {
          bearer: request.headers.get("authorization"),
          cliToken: request.headers.get("x-9r-cli-token"),
          cookie: request.headers.get("cookie"),
        };
        const body = (await request.json().catch(() => ({}))) as { name?: string };
        if (!body.name) {
          return Response.json({ error: "Name is required" }, { status: 400 });
        }
        state.createdKeyNames.push(body.name);
        return Response.json(
          {
            key: "sk-a1b2c3d4e5f6a7b8-k3y123-cafe1234",
            name: body.name,
            id: "key-uuid-001",
            machineId: "a1b2c3d4e5f6a7b8",
          },
          { status: 201 },
        );
      }

      // DELETE /api/keys/{id}
      const keyMatch = /^\/api\/keys\/([^/]+)$/.exec(url.pathname);
      if (keyMatch && request.method === "DELETE") {
        state.keysAuth = {
          bearer: request.headers.get("authorization"),
          cliToken: request.headers.get("x-9r-cli-token"),
          cookie: request.headers.get("cookie"),
        };
        state.revokedKeyIds.push(decodeURIComponent(keyMatch[1] ?? ""));
        return Response.json({ message: "Key deleted successfully" });
      }

      return Response.json({ error: { message: `No route: ${url.pathname}` } }, { status: 404 });
    },
  });

  return {
    url: server.url.toString().replace(/\/$/, ""),
    state,
    close() {
      server.stop(true);
    },
  };
}
