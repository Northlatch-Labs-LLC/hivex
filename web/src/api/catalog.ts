/**
 * Custom inference providers (Settings → Credentials → API Keys) and the
 * skills/experts/plugins marketplace. Extracted from client.ts to keep that
 * file inside the file-size budget (scripts/check-file-size.sh, 1500-LOC
 * ceiling). Re-exported by client.ts so import sites are unchanged.
 */

import { del, get, post, put } from "./client";

// ── Custom providers (Settings) ──

export interface CustomProvider {
  id: string;
  name: string;
  base_url: string;
  model: string;
  api_key?: string;
  enabled: boolean;
}

export function getCustomProviders() {
  return get<{ providers: CustomProvider[] }>("/custom-providers");
}

export function addCustomProvider(cp: CustomProvider) {
  return post<{ providers: CustomProvider[] }>("/custom-providers/add", cp);
}

export function updateCustomProvider(cp: CustomProvider) {
  return put<{ providers: CustomProvider[] }>("/custom-providers/update", cp);
}

export function deleteCustomProvider(id: string) {
  return del<{ providers: CustomProvider[] }>(`/custom-providers/delete/${id}`);
}

export function testCustomProvider(opts: {
  base_url: string;
  api_key?: string;
}) {
  return post<{ ok: boolean; status?: number; error?: string }>(
    "/custom-providers/test",
    opts,
  );
}

// ── Marketplace (skills, experts, plugins) ──

export interface MarketplaceEntry {
  id: string;
  category: "skill" | "expert" | "plugin";
  name: string;
  description: string;
  installed: boolean;
}

export function getMarketplace() {
  return get<{ entries: MarketplaceEntry[] }>("/marketplace");
}

export function installMarketplaceEntry(id: string) {
  return post<{ entries: MarketplaceEntry[] }>("/marketplace/install", { id });
}

export function uninstallMarketplaceEntry(id: string) {
  return post<{ entries: MarketplaceEntry[] }>("/marketplace/uninstall", {
    id,
  });
}
