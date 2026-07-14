import { useEffect, useState, useCallback } from "react";
import { ArrowLeft, Save } from "lucide-react";
import { api, ApiError } from "../api";
import type { RuntimeConfig } from "../types";

interface SettingsProps {
  onBack: () => void;
}

type LoadState =
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "ready"; config: RuntimeConfig };

type SaveState =
  | { kind: "idle" }
  | { kind: "saving" }
  | { kind: "success"; message: string }
  | { kind: "error"; message: string };

export function Settings({ onBack }: SettingsProps) {
  const [load, setLoad] = useState<LoadState>({ kind: "loading" });
  const [form, setForm] = useState<RuntimeConfig | null>(null);
  const [runActive, setRunActive] = useState(false);
  const [save, setSave] = useState<SaveState>({ kind: "idle" });

  useEffect(() => {
    const controller = new AbortController();
    api
      .getConfig(controller.signal)
      .then((config) => {
        setForm(config);
        setLoad({ kind: "ready", config });
      })
      .catch((err) => {
        setLoad({
          kind: "error",
          message: err instanceof Error ? err.message : "Failed to load configuration",
        });
      });
    return () => controller.abort();
  }, []);

  useEffect(() => {
    let cancelled = false;
    const poll = () => {
      api
        .activeRun()
        .then((res) => {
          if (!cancelled) setRunActive(res.runId !== null);
        })
        .catch(() => {});
    };
    poll();
    const id = setInterval(poll, 3000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  const update = useCallback((patch: Partial<RuntimeConfig>) => {
    setForm((prev) => (prev ? { ...prev, ...patch } : prev));
    setSave({ kind: "idle" });
  }, []);

  const handleSave = useCallback(() => {
    if (!form || runActive) return;
    setSave({ kind: "saving" });
    api
      .updateConfig(form)
      .then((updated) => {
        setForm(updated);
        setSave({ kind: "success", message: "Configuration saved." });
      })
      .catch((err) => {
        const message =
          err instanceof ApiError && err.status === 409
            ? "Cannot update configuration while a run is active."
            : err instanceof Error
              ? err.message
              : "Failed to save configuration";
        setSave({ kind: "error", message });
      });
  }, [form, runActive]);

  if (load.kind === "loading") {
    return (
      <div className="space-y-6">
        <div className="rounded-xl border bg-white p-6 shadow-sm">
          <div className="flex items-center gap-2 text-gray-500">
            <span className="w-4 h-4 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
            Loading configuration…
          </div>
        </div>
      </div>
    );
  }

  if (load.kind === "error") {
    return (
      <div className="space-y-6">
        <div className="rounded-xl border bg-white p-6 shadow-sm">
          <div className="rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700">
            {load.message}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="rounded-xl border bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <button
              onClick={onBack}
              className="flex items-center gap-1 text-sm text-gray-600 hover:text-gray-900"
            >
              <ArrowLeft size={16} />
              Back to Dashboard
            </button>
            <h2 className="text-lg font-semibold">Settings</h2>
          </div>
          <button
            onClick={handleSave}
            disabled={!form || runActive || save.kind === "saving"}
            className="flex items-center gap-2 rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Save size={16} />
            {save.kind === "saving" ? "Saving…" : "Save"}
          </button>
        </div>

        {runActive && (
          <div className="mt-4 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
            A benchmark run is currently active. Configuration changes are disabled until it
            completes or is stopped.
          </div>
        )}

        {save.kind === "success" && (
          <div className="mt-4 rounded-md border border-green-200 bg-green-50 p-3 text-sm text-green-800">
            {save.message}
          </div>
        )}

        {save.kind === "error" && (
          <div className="mt-4 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            {save.message}
          </div>
        )}

        {form && (
          <div className="mt-6 space-y-5">
            <Field
              label="Local Endpoint"
              hint="OpenAI-compatible endpoint for local models (e.g. http://localhost:8080). Leave empty to disable local discovery."
              value={form.localEndpoint}
              onChange={(v) => update({ localEndpoint: v })}
            />

            <Field
              label="Remote Endpoint"
              hint="OpenAI-compatible endpoint for remote models (e.g. https://api.openai.com). Leave empty to rely on the static model list."
              value={form.remoteEndpoint}
              onChange={(v) => update({ remoteEndpoint: v })}
            />

            <Field
              label="Remote API Key"
              hint="API key sent as Bearer token to the remote endpoint. Leave empty if the remote endpoint does not require authentication."
              value={form.remoteApiKey}
              type="password"
              onChange={(v) => update({ remoteApiKey: v })}
            />

            <div>
              <label className="block text-sm font-medium text-gray-900">
                Remote Models
              </label>
              <p className="mt-0.5 text-xs text-gray-500">
                Static model IDs to offer even when the remote endpoint is unreachable or returns
                no results. One ID per line.
              </p>
              <textarea
                className="mt-2 w-full rounded-md border border-gray-300 px-3 py-2 text-sm font-mono focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                rows={4}
                value={form.remoteModels.join("\n")}
                onChange={(e) =>
                  update({
                    remoteModels: e.target.value.split("\n").map((s) => s.trim()).filter(Boolean),
                  })
                }
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-900">
                Remote Model Cache TTL (seconds)
              </label>
              <p className="mt-0.5 text-xs text-gray-500">
                How long discovered remote model lists are cached before re-fetching.
              </p>
              <input
                type="number"
                className="mt-2 w-32 rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                value={form.remoteModelCacheTTLSeconds}
                onChange={(e) =>
                  update({
                    remoteModelCacheTTLSeconds: parseInt(e.target.value, 10) || 0,
                  })
                }
              />
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function Field({
  label,
  hint,
  value,
  onChange,
  type = "text",
}: {
  label: string;
  hint?: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
}) {
  return (
    <div>
      <label className="block text-sm font-medium text-gray-900">{label}</label>
      {hint && <p className="mt-0.5 text-xs text-gray-500">{hint}</p>}
      <input
        type={type}
        className="mt-2 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    </div>
  );
}
