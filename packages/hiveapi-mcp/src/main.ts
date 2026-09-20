/**
 * CLI entry — validates the env contract, then serves MCP over stdio.
 * A missing HIVEAPI_BASE_URL/HIVEAPI_API_KEY exits with a clear message
 * instead of starting a broken session.
 */

import { loadEnv, MissingEnvError } from "./env.ts";
import { runServer } from "./server.ts";

export async function main(): Promise<void> {
  let env;
  try {
    env = loadEnv();
  } catch (error) {
    if (error instanceof MissingEnvError) {
      console.error(
        `hiveapi-mcp: ${error.message}\n` +
          "  HIVEAPI_BASE_URL  e.g. http://127.0.0.1:8317\n" +
          "  HIVEAPI_API_KEY   gateway key (sk-…), optional only for a local no-auth gateway",
      );
      process.exit(1);
    }
    throw error;
  }
  await runServer(env);
}
