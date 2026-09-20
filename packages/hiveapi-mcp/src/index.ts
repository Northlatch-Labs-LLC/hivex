/**
 * @hiveapi/mcp — MCP server for the HiveAPI Gateway.
 *
 * @packageDocumentation
 */

export {
  createServer,
  runServer,
  SERVER_NAME,
  SERVER_VERSION,
} from "./server.ts";

export {
  loadEnv,
  MissingEnvError,
  type HiveEnv,
} from "./env.ts";

export {
  signEntitlementToken,
  verifyEntitlementToken,
  type EntitlementPayload,
  type VerifyResult,
} from "./entitlement.ts";

export {
  adminCreateKey,
  adminRevokeKey,
  fetchEntitlement,
  fetchUsage,
  reportTurns,
  GatewayHTTPError,
  type AdminCreateKeyResponse,
  type AdminDeleteKeyResponse,
  type DailyUsage,
  type EntitlementResponse,
  type PortalKeyUsage,
  type PortalUsageResponse,
  type TurnBudget,
  type TurnsReportResponse,
  type UsageTotals,
} from "./gateway.ts";

export {
  ADMIN_GATING_MESSAGE,
  PORTAL_COOKIE_HINT,
  toolDefinitions,
  hiveapiModels,
  hiveapiUsage,
  hiveapiEntitlement,
  hiveapiKeyCreate,
  hiveapiKeyRevoke,
  hiveapiTurnsReport,
  type McpToolHandler,
  type McpToolResult,
  type ToolDefinition,
} from "./tools.ts";
