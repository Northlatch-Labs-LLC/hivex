/**
 * MCP server wiring: registers the HiveAPI tools on an McpServer and
 * connects a stdio transport. Handlers are transport-independent — tests
 * exercise them directly or through InMemoryTransport pairs.
 */

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import type { z } from "zod";

import { loadEnv, type HiveEnv } from "./env.ts";
import { toolDefinitions, type McpToolResult } from "./tools.ts";

export const SERVER_NAME = "@hiveapi/mcp";
export const SERVER_VERSION = "0.1.0";

/**
 * Narrow structural view of McpServer#registerTool used for registration.
 * The SDK's real signature carries zod-compat generics that recurse deeply
 * (TS2589) when fed the registry-wide `Record<string, z.ZodTypeAny>` shape;
 * this view keeps the exact same runtime call with a flat type.
 */
type RegisterToolFn = (
  name: string,
  config: {
    title?: string;
    description?: string;
    inputSchema: Record<string, z.ZodTypeAny>;
  },
  cb: (args: Record<string, unknown>) => Promise<McpToolResult>,
) => unknown;

/** Build an McpServer with every HiveAPI tool registered. */
export function createServer(env: HiveEnv): McpServer {
  const server = new McpServer({
    name: SERVER_NAME,
    title: "HiveAPI Gateway",
    version: SERVER_VERSION,
  });

  const registerTool = server.registerTool.bind(server) as unknown as RegisterToolFn;
  for (const definition of toolDefinitions) {
    registerTool(
      definition.name,
      {
        title: definition.title,
        description: definition.description,
        inputSchema: definition.inputSchema,
      },
      (args) => definition.handler(args, env),
    );
  }
  return server;
}

/** Wire the server to stdio and start serving. Resolves once connected. */
export async function runServer(
  env?: HiveEnv,
  transport?: StdioServerTransport,
): Promise<McpServer> {
  const resolved = env ?? loadEnv();
  const server = createServer(resolved);
  await server.connect(transport ?? new StdioServerTransport());
  return server;
}
