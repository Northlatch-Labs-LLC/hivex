/**
 * MCP tool definitions for the HiveAPI Gateway.
 *
 * Handlers are plain async functions (McpToolHandler) so they can be tested
 * without a transport; server.ts registers them on an McpServer.
 */

import { HiveAPIClient, HiveAPIError } from "@hiveapi/client";
import { z } from "zod";

import type { HiveEnv } from "./env.ts";
import { verifyEntitlementToken, type VerifyResult } from "./entitlement.ts";
import {
  adminCreateKey,
  adminRevokeKey,
  fetchEntitlement,
  fetchUsage,
  GatewayHTTPError,
  reportTurns,
} from "./gateway.ts";

export const ADMIN_GATING_MESSAGE =
  "admin gating not configured — set HIVEAPI_ADMIN_TOKEN to enable this tool";

/** Message shown when a portal-session route is called without HIVEAPI_PORTAL_COOKIE. */
export const PORTAL_COOKIE_HINT =
  "HIVEAPI_PORTAL_COOKIE is not set — this gateway route requires a portal session " +
  "(copy the auth_token cookie value from a logged-in portal browser session)";

export interface McpToolResult {
  /** MCP ToolResult compatibility: the wire object allows extra fields. */
  [key: string]: unknown;
  content: Array<{ type: "text"; text: string }>;
  isError?: boolean;
}

export interface McpToolHandler<Args> {
  (args: Args, env: HiveEnv): Promise<McpToolResult>;
}

function text(body: unknown): McpToolResult {
  return { content: [{ type: "text", text: JSON.stringify(body, null, 2) }] };
}

function errorResult(error: unknown): McpToolResult {
  if (error instanceof GatewayHTTPError) {
    return {
      isError: true,
      content: [
        {
          type: "text",
          text: `Gateway error (HTTP ${error.status}): ${error.message}`,
        },
      ],
    };
  }
  if (error instanceof HiveAPIError) {
    return {
      isError: true,
      content: [
        {
          type: "text",
          text: `HiveAPI error (${error.kind}${error.status ? ` HTTP ${error.status}` : ""}): ${error.message}`,
        },
      ],
    };
  }
  return {
    isError: true,
    content: [{ type: "text", text: `Unexpected error: ${String(error)}` }],
  };
}

// ---------------------------------------------------------------------------
// hiveapi_models — list models via @hiveapi/client (GET /v1/models)
// ---------------------------------------------------------------------------

export const hiveapiModelsInput = {};

export const hiveapiModels: McpToolHandler<Record<string, never>> = async (_args, env) => {
  try {
    const client = new HiveAPIClient({ baseURL: env.baseURL, apiKey: env.apiKey });
    const models = await client.models.list();
    return text({
      count: models.data.length,
      models: models.data.map((m) => {
        const entry: Record<string, unknown> = { id: m.id, owned_by: m.owned_by };
        if (m.kind !== undefined) entry.kind = m.kind;
        if (m.context_length !== undefined) entry.context_length = m.context_length;
        if (m.max_completion_tokens !== undefined) entry.max_completion_tokens = m.max_completion_tokens;
        return entry;
      }),
    });
  } catch (error) {
    return errorResult(error);
  }
};

// ---------------------------------------------------------------------------
// hiveapi_usage — GET /api/portal/usage (per-key monthly usage + turn budget)
// ---------------------------------------------------------------------------

export const hiveapiUsageInput = {};

export const hiveapiUsage: McpToolHandler<Record<string, never>> = async (_args, env) => {
  if (!env.portalCookie) {
    return {
      content: [{ type: "text", text: PORTAL_COOKIE_HINT }],
      isError: true,
    };
  }
  try {
    const usage = await fetchUsage(env);
    return text({
      monthStart: usage.monthStart,
      tier: usage.tier,
      totals: usage.totals,
      turns: usage.turns,
      keys: usage.keys,
      daily: usage.daily,
    });
  } catch (error) {
    return errorResult(error);
  }
};

// ---------------------------------------------------------------------------
// hiveapi_entitlement — GET /api/portal/entitlement + local HMAC verification
// ---------------------------------------------------------------------------

export const hiveapiEntitlementInput = {};

export const hiveapiEntitlement: McpToolHandler<Record<string, never>> = async (_args, env) => {
  if (!env.portalCookie) {
    return {
      content: [{ type: "text", text: PORTAL_COOKIE_HINT }],
      isError: true,
    };
  }
  try {
    const { token, entitlement } = await fetchEntitlement(env);
    const body: Record<string, unknown> = { entitlement };

    if (!env.entitlementSecret) {
      body.verified = false;
      body.verification = "not configured — set HIVEAPI_ENTITLEMENT_SECRET (must match the gateway's ENTITLEMENT_SIGNING_KEY) to verify the HMAC";
    } else {
      const result: VerifyResult = verifyEntitlementToken(token, env.entitlementSecret);
      if (result.ok) {
        body.verified = true;
        body.verification = "HMAC-SHA256 signature valid and not expired";
      } else {
        body.verified = false;
        body.verification = `rejected (${result.reason})`;
        return { content: [{ type: "text", text: JSON.stringify(body, null, 2) }], isError: true };
      }
    }
    return text(body);
  } catch (error) {
    return errorResult(error);
  }
};

// ---------------------------------------------------------------------------
// hiveapi_key_create — POST /api/keys (ADMIN-gated)
// ---------------------------------------------------------------------------

export const hiveapiKeyCreateInput = {
  name: z.string().min(1).describe("Display name for the new gateway API key"),
};

export const hiveapiKeyCreate: McpToolHandler<{ name: string }> = async (args, env) => {
  if (!env.adminToken) {
    return { content: [{ type: "text", text: ADMIN_GATING_MESSAGE }] };
  }
  try {
    const created = await adminCreateKey(env, args.name);
    return text({
      created: true,
      id: created.id,
      name: created.name,
      machineId: created.machineId,
      // The full key value is returned by the gateway exactly once.
      key: created.key,
      warning: "Store this key now — the gateway never returns the full value again.",
    });
  } catch (error) {
    return errorResult(error);
  }
};

// ---------------------------------------------------------------------------
// hiveapi_key_revoke — DELETE /api/keys/{id} (ADMIN-gated)
// ---------------------------------------------------------------------------

export const hiveapiKeyRevokeInput = {
  id: z.string().min(1).describe("API key id to revoke (from POST /api/keys or the dashboard)"),
};

export const hiveapiKeyRevoke: McpToolHandler<{ id: string }> = async (args, env) => {
  if (!env.adminToken) {
    return { content: [{ type: "text", text: ADMIN_GATING_MESSAGE }] };
  }
  try {
    const deleted = await adminRevokeKey(env, args.id);
    return text({ revoked: true, id: args.id, message: deleted.message });
  } catch (error) {
    return errorResult(error);
  }
};

// ---------------------------------------------------------------------------
// hiveapi_turns_report — POST /api/portal/turns (account key auth)
// ---------------------------------------------------------------------------

export const hiveapiTurnsReportInput = {
  turns: z.number().int().positive().default(1).describe("Number of harness turns to record (default 1)"),
};

export const hiveapiTurnsReport: McpToolHandler<{ turns: number }> = async (args, env) => {
  try {
    const result = await reportTurns(env, args.turns);
    return text(result);
  } catch (error) {
    return errorResult(error);
  }
};

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

export interface ToolDefinition {
  name: string;
  title: string;
  description: string;
  /** Zod raw shape for MCP input validation. */
  inputSchema: Record<string, z.ZodTypeAny>;
  handler: McpToolHandler<any>;
}

export const toolDefinitions: ToolDefinition[] = [
  {
    name: "hiveapi_models",
    title: "List HiveAPI models",
    description:
      "List the chat/LLM models available on the HiveAPI Gateway (GET /v1/models). " +
      "Returns model ids (\"owner/model\"), owners, and context-window metadata.",
    inputSchema: hiveapiModelsInput,
    handler: hiveapiModels,
  },
  {
    name: "hiveapi_usage",
    title: "HiveAPI usage summary",
    description:
      "Fetch the caller's per-key gateway usage for the current month plus the harness turn " +
      "budget (GET /api/portal/usage — requires HIVEAPI_PORTAL_COOKIE, the portal session cookie).",
    inputSchema: hiveapiUsageInput,
    handler: hiveapiUsage,
  },
  {
    name: "hiveapi_entitlement",
    title: "Fetch and verify entitlement",
    description:
      "Fetch the account's signed entitlement token (GET /api/portal/entitlement — requires " +
      "HIVEAPI_PORTAL_COOKIE) and verify its base64url HMAC-SHA256 signature with " +
      "HIVEAPI_ENTITLEMENT_SECRET. Reports { account, tier, turn_cap_monthly, provider_locked, exp }.",
    inputSchema: hiveapiEntitlementInput,
    handler: hiveapiEntitlement,
  },
  {
    name: "hiveapi_key_create",
    title: "Create gateway API key (admin)",
    description:
      "Create a new gateway API key (POST /api/keys, body {name}). ADMIN-gated: only enabled when " +
      "HIVEAPI_ADMIN_TOKEN is set; without it the tool answers that admin gating is not configured.",
    inputSchema: hiveapiKeyCreateInput,
    handler: hiveapiKeyCreate,
  },
  {
    name: "hiveapi_key_revoke",
    title: "Revoke gateway API key (admin)",
    description:
      "Delete (revoke) a gateway API key by id (DELETE /api/keys/{id}). ADMIN-gated: only enabled " +
      "when HIVEAPI_ADMIN_TOKEN is set; without it the tool answers that admin gating is not configured.",
    inputSchema: hiveapiKeyRevokeInput,
    handler: hiveapiKeyRevoke,
  },
  {
    name: "hiveapi_turns_report",
    title: "Report harness turns",
    description:
      "Report harness turn metering to the gateway (POST /api/portal/turns, body {turns}) with the " +
      "account's gateway API key as Bearer auth. Returns the turn budget: " +
      "{ success, allowed, used, cap, capped, remaining }.",
    inputSchema: hiveapiTurnsReportInput,
    handler: hiveapiTurnsReport,
  },
];
