import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  addCustomProvider,
  type CustomProvider,
  deleteCustomProvider,
  getCustomProviders,
  testCustomProvider,
} from "../../api/client";
import { showNotice } from "../ui/Toast";

const labelStyle = {
  display: "block",
  fontSize: 11,
  fontWeight: 600,
  textTransform: "uppercase",
  letterSpacing: "0.05em",
  color: "var(--text-tertiary)",
  marginBottom: 6,
} as const;

const inputStyle = {
  width: "100%",
  padding: "8px 10px",
  fontSize: 13,
  background: "var(--bg-warm)",
  color: "var(--text)",
  border: "1px solid var(--border)",
  borderRadius: 6,
  fontFamily: "var(--font-mono)",
} as const;

const emptyForm = { id: "", name: "", base_url: "", model: "", api_key: "" };

// CustomProvidersSection is the Settings surface for user-defined
// OpenAI-compatible providers: add/list/test/delete, persisted by the broker.
export function CustomProvidersSection() {
  const qc = useQueryClient();
  const [form, setForm] = useState(emptyForm);
  const [testResult, setTestResult] = useState<string | null>(null);

  const list = useQuery({
    queryKey: ["custom-providers"],
    queryFn: getCustomProviders,
  });

  const invalidate = {
    onSuccess: () => qc.invalidateQueries({ queryKey: ["custom-providers"] }),
  };

  const add = useMutation({
    mutationFn: () => addCustomProvider({ ...form, enabled: true }),
    ...invalidate,
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteCustomProvider(id),
    ...invalidate,
  });
  const test = useMutation({
    mutationFn: () =>
      testCustomProvider({ base_url: form.base_url, api_key: form.api_key }),
    onSuccess: (r) =>
      setTestResult(
        r.ok
          ? `reachable (HTTP ${r.status ?? 200})`
          : `unreachable: ${r.error ?? `HTTP ${r.status}`}`,
      ),
  });

  const set =
    (k: keyof typeof form) => (e: React.ChangeEvent<HTMLInputElement>) =>
      setForm((f) => ({ ...f, [k]: e.target.value }));

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    add.mutate(undefined, {
      onSuccess: () => setForm(emptyForm),
      onError: (err: Error) => showNotice(err.message),
    });
  };

  const providers: CustomProvider[] = list.data?.providers ?? [];

  return (
    <div>
      <h2>Custom providers</h2>
      <p style={{ fontSize: 13, color: "var(--text-secondary)" }}>
        Any OpenAI-compatible endpoint: base URL, model, API key. Managed here,
        used by the harness.
      </p>
      <form
        onSubmit={submit}
        style={{ display: "grid", gap: 10, maxWidth: 420, marginTop: 12 }}
      >
        <label style={labelStyle}>
          Name
          <label
            style={{
              ...labelStyle,
              textTransform: "none",
              color: "var(--text)",
            }}
          >
            <input
              style={inputStyle}
              value={form.name}
              onChange={set("name")}
              placeholder="My provider"
              required={true}
            />
          </label>
        </label>
        <label style={labelStyle}>
          ID
          <label
            style={{
              ...labelStyle,
              textTransform: "none",
              color: "var(--text)",
            }}
          >
            <input
              style={inputStyle}
              value={form.id}
              onChange={set("id")}
              placeholder="custom-my-provider"
              required={true}
            />
          </label>
        </label>
        <label style={labelStyle}>
          Base URL
          <label
            style={{
              ...labelStyle,
              textTransform: "none",
              color: "var(--text)",
            }}
          >
            <input
              style={inputStyle}
              value={form.base_url}
              onChange={set("base_url")}
              placeholder="https://api.example.com/v1"
              required={true}
            />
          </label>
        </label>
        <label style={labelStyle}>
          Model
          <label
            style={{
              ...labelStyle,
              textTransform: "none",
              color: "var(--text)",
            }}
          >
            <input
              style={inputStyle}
              value={form.model}
              onChange={set("model")}
              placeholder="glm-4.7"
              required={true}
            />
          </label>
        </label>
        <label style={labelStyle}>
          API key
          <label
            style={{
              ...labelStyle,
              textTransform: "none",
              color: "var(--text)",
            }}
          >
            <input
              style={inputStyle}
              type="password"
              value={form.api_key}
              onChange={set("api_key")}
            />
          </label>
        </label>
        <div style={{ display: "flex", gap: 8 }}>
          <button type="submit" disabled={add.isPending}>
            Add provider
          </button>
          <button
            type="button"
            onClick={() => test.mutate()}
            disabled={test.isPending || !form.base_url}
          >
            Test
          </button>
        </div>
        {testResult ? (
          <p style={{ fontSize: 12, color: "var(--text-tertiary)" }}>
            {testResult}
          </p>
        ) : null}
      </form>
      {providers.length > 0 && (
        <ul
          style={{
            marginTop: 16,
            display: "grid",
            gap: 8,
            padding: 0,
            listStyle: "none",
          }}
        >
          {providers.map((cp) => (
            <li
              key={cp.id}
              style={{
                display: "flex",
                gap: 12,
                alignItems: "center",
                fontSize: 13,
              }}
            >
              <strong>{cp.name}</strong>
              <span style={{ color: "var(--text-tertiary)" }}>
                {cp.id} · {cp.base_url} · {cp.model}
              </span>
              <button
                type="button"
                onClick={() => remove.mutate(cp.id)}
                disabled={remove.isPending}
              >
                Remove
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
