import { useEffect, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";

import {
  get,
  getCustomProviders,
  getLocalProvidersStatus,
  type LLMRuntimeKind,
} from "../../../api/client";
import {
  prereqByBinary,
  RUNTIME_PROVIDER_OPTIONS,
  runtimeProviderIsConnected,
  runtimeProviderLabel,
  statusByLocalProvider,
} from "../../../lib/runtimeProviders";
import type { PrereqResult } from "../../onboarding/runtimes";
import { Field } from "./components";

interface RuntimeProviderChecklistProps {
  configuredKinds?: readonly string[];
  selectedProviders: string[];
  onSelectedProvidersChange: (providers: string[]) => void;
  onConnectedProvidersChange: (providers: string[]) => void;
}

export function RuntimeProviderChecklist({
  configuredKinds,
  selectedProviders,
  onSelectedProvidersChange,
  onConnectedProvidersChange,
}: RuntimeProviderChecklistProps) {
  const prereqs = useQuery({
    queryKey: ["settings-runtime-prereqs"],
    queryFn: () =>
      get<{ prereqs?: PrereqResult[] } | PrereqResult[]>("/onboarding/prereqs"),
    staleTime: 10_000,
  });
  const localStatuses = useQuery({
    queryKey: ["settings-runtime-local-providers"],
    queryFn: getLocalProvidersStatus,
    staleTime: 10_000,
  });
  const customProviders = useQuery({
    queryKey: ["custom-providers"],
    queryFn: getCustomProviders,
    staleTime: 10_000,
  });

  const prereqList = useMemo(
    () =>
      Array.isArray(prereqs.data)
        ? prereqs.data
        : (prereqs.data?.prereqs ?? []),
    [prereqs.data],
  );
  const prereqMap = useMemo(() => prereqByBinary(prereqList), [prereqList]);
  const localStatusMap = useMemo(
    () => statusByLocalProvider(localStatuses.data),
    [localStatuses.data],
  );
  const runtimeKindSet = useMemo(
    () => new Set(configuredKinds ?? []),
    [configuredKinds],
  );
  const providerOptions = useMemo(
    () =>
      runtimeKindSet.size > 0
        ? RUNTIME_PROVIDER_OPTIONS.filter((option) =>
            runtimeKindSet.has(option.id),
          )
        : RUNTIME_PROVIDER_OPTIONS,
    [runtimeKindSet],
  );

  const connectedProviders = useMemo(() => {
    const builtIn = selectedProviders.filter((id): id is LLMRuntimeKind => {
      const option = RUNTIME_PROVIDER_OPTIONS.find((p) => p.id === id);
      return option
        ? runtimeProviderIsConnected(option, {
            prereqs: prereqMap,
            localStatuses: localStatusMap,
          })
        : false;
    });
    // Settings-managed custom providers are verified by the user at add
    // time (Check connection) — enabled means selectable.
    const customIds = (customProviders.data?.providers ?? [])
      .filter((cp) => cp.enabled && selectedProviders.includes(cp.id))
      .map((cp) => cp.id);
    return [...builtIn, ...customIds];
  }, [customProviders.data, localStatusMap, prereqMap, selectedProviders]);

  useEffect(() => {
    if (prereqs.isError || localStatuses.isError) return;
    if (prereqs.data === undefined || localStatuses.data === undefined) return;
    onConnectedProvidersChange(connectedProviders);
  }, [
    connectedProviders,
    localStatuses.data,
    localStatuses.isError,
    onConnectedProvidersChange,
    prereqs.data,
    prereqs.isError,
  ]);

  function toggleProvider(id: string): void {
    onSelectedProvidersChange(
      selectedProviders.includes(id)
        ? selectedProviders.filter((p) => p !== id)
        : [...selectedProviders, id],
    );
  }

  return (
    <>
      <div style={{ marginTop: 24, fontSize: 14, fontWeight: 600 }}>
        Default runtime for new bots
      </div>
      <p
        style={{
          fontSize: 12,
          color: "var(--text-tertiary)",
          margin: "0 0 12px 0",
          lineHeight: 1.5,
        }}
      >
        Picked for new agents at creation time. Existing agents keep whatever
        runtime they already have - change those one at a time from each agent's
        profile (Runtime section). To import OpenClaw or Hermes agents into the
        team, use the Integrations app - those gateways are not runtimes you
        assign here.
      </p>
      <Field label="Available providers" hint="Verified runtimes">
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {providerOptions.map((option) => {
            const connected = runtimeProviderIsConnected(option, {
              prereqs: prereqMap,
              localStatuses: localStatusMap,
            });
            const checked = selectedProviders.includes(option.id);
            return (
              <label
                key={option.id}
                style={{
                  display: "flex",
                  gap: 10,
                  alignItems: "flex-start",
                  padding: "10px 12px",
                  border: "1px solid var(--border)",
                  borderRadius: "var(--radius-sm)",
                  background: checked ? "var(--accent-bg)" : "var(--bg-card)",
                  opacity: connected ? 1 : 0.58,
                }}
              >
                <input
                  type="checkbox"
                  checked={checked}
                  disabled={!connected}
                  onChange={() => toggleProvider(option.id)}
                  style={{ marginTop: 2 }}
                />
                <span>
                  <span
                    style={{ display: "block", fontSize: 13, fontWeight: 600 }}
                  >
                    {option.label}
                  </span>
                  <span
                    style={{
                      display: "block",
                      fontSize: 11,
                      color: "var(--text-tertiary)",
                      lineHeight: 1.4,
                    }}
                  >
                    {connected ? option.desc : "Not connected or not running"}
                  </span>
                </span>
              </label>
            );
          })}
          {(customProviders.data?.providers ?? []).map((cp) => {
            const checked = selectedProviders.includes(cp.id);
            return (
              <label
                key={cp.id}
                style={{
                  display: "flex",
                  gap: 10,
                  alignItems: "flex-start",
                  padding: "10px 12px",
                  border: "1px solid var(--border)",
                  borderRadius: "var(--radius-sm)",
                  background: checked ? "var(--accent-bg)" : "var(--bg-card)",
                  opacity: cp.enabled ? 1 : 0.58,
                }}
              >
                <input
                  type="checkbox"
                  checked={checked}
                  disabled={!cp.enabled}
                  onChange={() => toggleProvider(cp.id)}
                  style={{ marginTop: 2 }}
                />
                <span>
                  <span
                    style={{ display: "block", fontSize: 13, fontWeight: 600 }}
                  >
                    {cp.name}
                  </span>
                  <span
                    style={{
                      display: "block",
                      fontSize: 11,
                      color: "var(--text-tertiary)",
                      lineHeight: 1.4,
                    }}
                  >
                    {cp.base_url} · {cp.model}
                  </span>
                </span>
              </label>
            );
          })}
          {selectedProviders.length > 0 ? (
            <div style={{ fontSize: 12, color: "var(--text-secondary)" }}>
              Task creation will show:{" "}
              {selectedProviders
                .map((id) => {
                  const cp = (customProviders.data?.providers ?? []).find(
                    (c) => c.id === id,
                  );
                  return cp
                    ? cp.name
                    : runtimeProviderLabel(id as LLMRuntimeKind);
                })
                .join(", ")}
            </div>
          ) : null}
        </div>
      </Field>
    </>
  );
}
