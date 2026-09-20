import { describe, expect, it } from "bun:test";

import {
  buildRequestURL,
  formatBaseURL,
  normalizeBaseURL,
} from "../src/url.ts";
import { HiveAPIClient } from "../src/client.ts";

describe("normalizeBaseURL — /v1 auto-append", () => {
  it("appends /v1 to a bare host", () => {
    expect(normalizeBaseURL("https://gw.example")).toEqual({
      origin: "https://gw.example",
      path: "/v1",
      search: "",
    });
  });

  it("appends /v1 to a host with a trailing slash", () => {
    expect(normalizeBaseURL("https://gw.example/").path).toBe("/v1");
  });

  it("keeps an explicit /v1", () => {
    expect(normalizeBaseURL("https://gw.example/v1").path).toBe("/v1");
  });

  it("normalises a trailing slash after /v1", () => {
    expect(normalizeBaseURL("https://gw.example/v1/").path).toBe("/v1");
  });

  it("appends /v1 after a path prefix", () => {
    expect(normalizeBaseURL("https://gw.example/gateway").path).toBe("/gateway/v1");
  });

  it("preserves ports", () => {
    expect(normalizeBaseURL("http://127.0.0.1:8317").origin).toBe("http://127.0.0.1:8317");
    expect(normalizeBaseURL("http://127.0.0.1:8317").path).toBe("/v1");
  });

  it("round-trips query-string base URLs", () => {
    const base = normalizeBaseURL("https://gw.example?token=abc&x=1");
    expect(base).toEqual({ origin: "https://gw.example", path: "/v1", search: "?token=abc&x=1" });
    expect(formatBaseURL(base)).toBe("https://gw.example/v1?token=abc&x=1");
  });

  it("round-trips a /v1 base with a query string", () => {
    expect(formatBaseURL(normalizeBaseURL("https://gw.example/v1?token=abc"))).toBe(
      "https://gw.example/v1?token=abc",
    );
  });

  it("rejects non-http(s) and malformed URLs", () => {
    expect(() => normalizeBaseURL("not a url")).toThrow();
    expect(() => normalizeBaseURL("ftp://gw.example")).toThrow();
  });
});

describe("buildRequestURL", () => {
  it("builds routes under /v1", () => {
    expect(buildRequestURL(normalizeBaseURL("https://gw.example"), "models")).toBe(
      "https://gw.example/v1/models",
    );
  });

  it("carries base query params on every request", () => {
    expect(
      buildRequestURL(normalizeBaseURL("https://gw.example?token=abc"), "chat/completions"),
    ).toBe("https://gw.example/v1/chat/completions?token=abc");
  });

  it("merges route query params after base query params", () => {
    const base = normalizeBaseURL("https://gw.example?token=abc");
    expect(buildRequestURL(base, "audio/speech?response_format=json")).toBe(
      "https://gw.example/v1/audio/speech?token=abc&response_format=json",
    );
  });

  it("uses route query params alone when the base has none", () => {
    expect(
      buildRequestURL(normalizeBaseURL("https://gw.example"), "images/generations?response_format=binary"),
    ).toBe("https://gw.example/v1/images/generations?response_format=binary");
  });

  it("keeps path prefixes before the query string", () => {
    const base = normalizeBaseURL("https://gw.example/gateway");
    expect(buildRequestURL(base, "search")).toBe("https://gw.example/gateway/v1/search");
  });
});

describe("HiveAPIClient baseURL", () => {
  it("exposes the normalised base URL", () => {
    const client = new HiveAPIClient({ baseURL: "https://gw.example", apiKey: "sk-test" });
    expect(client.baseURL).toBe("https://gw.example/v1");
  });

  it("round-trips a query-string base through the client", () => {
    const client = new HiveAPIClient({ baseURL: "https://gw.example?token=abc", apiKey: "sk-test" });
    expect(client.baseURL).toBe("https://gw.example/v1?token=abc");
  });
});
