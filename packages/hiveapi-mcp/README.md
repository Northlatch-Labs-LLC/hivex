# @hiveapi/mcp

MCP server for the **HiveAPI Gateway** — exposes the gateway's model catalog,
per-key usage, signed entitlement verification, harness turn reporting, and
admin-gated API key management as MCP tools over stdio.

Built on [`@modelcontextprotocol/sdk`](https://github.com/modelcontextprotocol/typescript-sdk),
served with the [`@hiveapi/client`](../hiveapi-client) package for the
OpenAI-compatible `/v1` surface.

## Tools

| Tool | Gateway route | Auth | Notes |
|---|---|---|---|
| `hiveapi_models` | `GET /v1/models` | API key | Lists chat models with context-window metadata. |
| `hiveapi_usage` | `GET /api/portal/usage` | portal session | Per-key monthly usage + harness turn budget (masked keys only). |
| `hiveapi_entitlement` | `GET /api/portal/entitlement` | portal session | Fetches `{token, entitlement}` and verifies the base64url HMAC-SHA256 signature locally. |
| `hiveapi_key_create` | `POST /api/keys` | admin | **ADMIN-gated** — answers "admin gating not configured" when `HIVEAPI_ADMIN_TOKEN` is unset. |
| `hiveapi_key_revoke` | `DELETE /api/keys/{id}` | admin | **ADMIN-gated** — same graceful refusal. |
| `hiveapi_turns_report` | `POST /api/portal/turns` | API key | Reports harness turns; returns `{success, allowed, used, cap, capped, remaining}`. |

## Environment contract

| Variable | Required | Meaning |
|---|---|---|
| `HIVEAPI_BASE_URL` | yes | Gateway base URL, e.g. `http://127.0.0.1:8317` (no `/v1` needed — the server appends it for `/v1` routes and uses the raw base for portal/admin routes). |
| `HIVEAPI_API_KEY` | yes* | Gateway API key (`sk-…`). *Optional only for a local no-auth gateway; always required for `hiveapi_turns_report`. |
| `HIVEAPI_PORTAL_COOKIE` | no | Portal session `auth_token` cookie value (from a logged-in portal browser session). Required in practice for `hiveapi_usage` and `hiveapi_entitlement` — those gateway routes only accept the session cookie. |
| `HIVEAPI_ENTITLEMENT_SECRET` | no | Secret shared with the gateway (`ENTITLEMENT_SIGNING_KEY`). Enables local HMAC verification of entitlement tokens. |
| `HIVEAPI_ADMIN_TOKEN` | no | Credential the gateway accepts for `/api/keys` (dashboard JWT or CLI token). When unset, the admin tools answer "admin gating not configured" instead of failing the session. |

## MCP client configuration

### Claude Code

```bash
claude mcp add hiveapi --env HIVEAPI_BASE_URL=http://127.0.0.1:8317 --env HIVEAPI_API_KEY=sk-your-key -- bun /abs/path/to/packages/hiveapi-mcp/bin/hiveapi-mcp
```

Or in `.mcp.json` / `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "hiveapi": {
      "command": "bun",
      "args": ["/abs/path/to/packages/hiveapi-mcp/bin/hiveapi-mcp"],
      "env": {
        "HIVEAPI_BASE_URL": "http://127.0.0.1:8317",
        "HIVEAPI_API_KEY": "sk-your-key",
        "HIVEAPI_ENTITLEMENT_SECRET": "same-value-as-ENTITLEMENT_SIGNING_KEY",
        "HIVEAPI_ADMIN_TOKEN": "dashboard-jwt-or-cli-token"
      }
    }
  }
}
```

### Cursor

`.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "hiveapi": {
      "command": "bun",
      "args": ["/abs/path/to/packages/hiveapi-mcp/bin/hiveapi-mcp"],
      "env": {
        "HIVEAPI_BASE_URL": "http://127.0.0.1:8317",
        "HIVEAPI_API_KEY": "sk-your-key"
      }
    }
  }
}
```

### ZCode

`.zcode/mcp.json` (project scope) or `~/.zcode/mcp.json` (user scope):

```json
{
  "mcpServers": {
    "hiveapi": {
      "command": "bun",
      "args": ["/abs/path/to/packages/hiveapi-mcp/bin/hiveapi-mcp"],
      "env": {
        "HIVEAPI_BASE_URL": "http://127.0.0.1:8317",
        "HIVEAPI_API_KEY": "sk-your-key",
        "HIVEAPI_ENTITLEMENT_SECRET": "same-value-as-ENTITLEMENT_SIGNING_KEY",
        "HIVEAPI_ADMIN_TOKEN": "dashboard-jwt-or-cli-token"
      }
    }
  }
}
```

### npx (published installs)

```json
{
  "mcpServers": {
    "hiveapi": {
      "command": "bunx",
      "args": ["hiveapi-mcp"],
      "env": {
        "HIVEAPI_BASE_URL": "http://127.0.0.1:8317",
        "HIVEAPI_API_KEY": "sk-your-key"
      }
    }
  }
}
```

The bin script is `#!/usr/bin/env bun`; run it with `bun` (or ensure `bun` is
on `PATH`).

## Security notes

- The admin token is sent on every auth channel the gateway accepts for
  `/api/keys`: `Authorization: Bearer`, `x-9r-cli-token` (dashboard CLI token),
  and the `auth_token` cookie.
- Entitlement verification uses constant-time HMAC comparison
  (`crypto.timingSafeEqual`) and rejects expired tokens (`exp` is in seconds).
- `hiveapi_key_create` returns the full `sk-{machineId}-{keyId}-{crc8}` key
  value exactly once — the gateway never shows it again (only masked forms).

## Development

```bash
bun install   # from the repo root (bun workspaces)
cd packages/hiveapi-mcp
bun test           # tool schema validation, admin-gated refusal, entitlement
                   # HMAC round-trip, turns-report happy path against a Bun.serve mock
bun run typecheck  # tsc --noEmit
```

## License

MIT — see [LICENSE](./LICENSE). Copyright (c) 2026 Northlatch Labs LLC.
