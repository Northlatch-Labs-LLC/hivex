/**
 * Base URL normalisation.
 *
 * The gateway serves its unified inference API under `{base}/v1` (rewritten to
 * /api/v1/* — see 9router/next.config.mjs). The client auto-appends `/v1`
 * when the configured base URL doesn't already end with it, and preserves
 * any query string so signed/scoped base URLs round-trip on every request.
 */

export interface NormalizedBase {
  origin: string;
  /** Path prefix ending in "/v1" (no trailing slash). */
  path: string;
  /** Raw query string including the leading "?", or "" when absent. */
  search: string;
}

export function normalizeBaseURL(baseURL: string): NormalizedBase {
  let url: URL;
  try {
    url = new URL(baseURL);
  } catch {
    throw new TypeError(
      `HiveAPI baseURL must be an absolute http(s) URL, got: ${JSON.stringify(baseURL)}`,
    );
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    throw new TypeError(
      `HiveAPI baseURL must use http or https, got protocol ${JSON.stringify(url.protocol)}`,
    );
  }

  let path = url.pathname;
  if (path.length > 1 && path.endsWith("/")) {
    path = path.slice(0, -1);
  }
  if (path === "/") {
    path = ""; // bare origin — append cleanly below
  }
  if (!path.endsWith("/v1")) {
    path += "/v1";
  }
  return { origin: url.origin, path, search: url.search };
}

/** Reproduce a full base URL string (used by HiveAPIClient#baseURL). */
export function formatBaseURL(base: NormalizedBase): string {
  return base.origin + base.path + base.search;
}

/**
 * Build an absolute request URL from the normalised base and a route.
 *
 * @param route  Route relative to the /v1 prefix, optionally with its own
 *               query string (e.g. "chat/completions" or
 *               "audio/speech?response_format=json"). Route query params are
 *               appended after any base query params.
 */
export function buildRequestURL(base: NormalizedBase, route: string): string {
  const q = route.indexOf("?");
  const routePath = q === -1 ? route : route.slice(0, q);
  const routeQuery = q === -1 ? "" : route.slice(q + 1);

  let search = base.search;
  if (routeQuery) {
    search = search ? `${search}&${routeQuery}` : `?${routeQuery}`;
  }
  const prefix = base.path === "/" ? "" : `${base.path}/`;
  return `${base.origin}${prefix}${routePath.replace(/^\/+/, "")}${search}`;
}
