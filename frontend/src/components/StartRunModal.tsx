import { useEffect, useRef, useState, type FormEvent, useMemo } from "react";
import { X, ChevronDown, ChevronRight } from "lucide-react";
import { api } from "../api";
import type { ScenarioInfo, Model, ModelsResponse } from "../types";

interface StartRunModalProps {
  onClose: () => void;
  onLaunch: (runId: string, scenarioIds: string[]) => void;
}

type LoadingState = { state: "loading" } | { state: "error"; message: string } | { state: "ready" };

export function StartRunModal({ onClose, onLaunch }: StartRunModalProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [scenarios, setScenarios] = useState<ScenarioInfo[]>([]);
  const [models, setModels] = useState<ModelsResponse>({ local: [], remote: [] });
  const [fetchState, setFetchState] = useState<LoadingState>({ state: "loading" });

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [selectedModel, setSelectedModel] = useState<string>("");
  const [systemPrompt, setSystemPrompt] = useState("");
  const [timeoutSecs, setTimeoutSecs] = useState(600);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();

    const load = async () => {
      try {
        const [sRes, mRes] = await Promise.all([
          api.getScenarios(controller.signal),
          api.getModels(controller.signal),
        ]);
        if (cancelled) return;
        setScenarios(sRes);
        setModels(mRes);
        setSelectedIds(new Set(sRes.map((s) => s.id)));
        const allModels = [...mRes.local, ...mRes.remote];
        if (allModels.length > 0) {
          setSelectedModel(allModels[0].id);
        }
        setFetchState({ state: "ready" });
      } catch (err) {
        if (cancelled) return;
        setFetchState({
          state: "error",
          message: err instanceof Error ? err.message : "Failed to load scenarios or models",
        });
      }
    };

    load();
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, []);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;
    if (!dialog.open) dialog.showModal();
    const onCancel = (e: Event) => {
      e.preventDefault();
      onClose();
    };
    dialog.addEventListener("cancel", onCancel);
    return () => {
      dialog.removeEventListener("cancel", onCancel);
      if (dialog.open) dialog.close();
    };
  }, [onClose]);

  const scenariosByCategory = useMemo(() => {
    return scenarios.reduce<Record<string, ScenarioInfo[]>>((acc, scenario) => {
      (acc[scenario.category] ??= []).push(scenario);
      return acc;
    }, {});
  }, [scenarios]);

  const allModelOptions: { model: Model; source: "local" | "remote" }[] = useMemo(
    () => [
      ...models.local.map((m) => ({ model: m, source: "local" as const })),
      ...models.remote.map((m) => ({ model: m, source: "remote" as const })),
    ],
    [models]
  );

  const toggleScenario = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const selectAll = (ids: string[]) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      ids.forEach((id) => next.add(id));
      return next;
    });
  };

  const clearGroup = (ids: string[]) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      ids.forEach((id) => next.delete(id));
      return next;
    });
  };

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (selectedIds.size === 0) {
      setSubmitError("Select at least one scenario");
      return;
    }
    if (!selectedModel) {
      setSubmitError("Select a model");
      return;
    }
    setSubmitError(null);
    setIsSubmitting(true);
    api
      .createRun({
        scenarioIds: [...selectedIds],
        modelId: selectedModel,
        systemPrompt: systemPrompt || undefined,
        timeoutMs: timeoutSecs > 0 ? timeoutSecs * 1000 : undefined,
      })
      .then(({ runId }) => {
        onLaunch(runId, [...selectedIds]);
      })
      .catch((err) => {
        setSubmitError(err instanceof Error ? err.message : "Failed to start run");
      })
      .finally(() => setIsSubmitting(false));
  };

  const handleDialogClick = (e: React.MouseEvent<HTMLDialogElement>) => {
    if (e.target === dialogRef.current) onClose();
  };

  return (
    <dialog
      ref={dialogRef}
      onClick={handleDialogClick}
      aria-labelledby="start-run-title"
      className="bg-white border border-gray-200 w-full max-w-2xl h-[85vh] p-0 m-auto text-sm text-gray-900 backdrop:bg-black/70 flex flex-col rounded-lg shadow-xl"
    >
      <form onSubmit={handleSubmit} className="flex flex-col h-full">
        <div className="flex justify-between items-center px-4 py-3 border-b border-gray-200 bg-gray-50 rounded-t-lg">
          <span id="start-run-title" className="font-bold uppercase tracking-wider text-[11px] text-gray-700">
            Start Run
          </span>
          <button type="button" onClick={onClose} className="text-gray-400 hover:text-gray-600">
            <X size={16} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-4 min-h-0">
          {fetchState.state === "loading" ? (
            <div className="text-gray-500 text-center py-8">Loading…</div>
          ) : fetchState.state === "error" ? (
            <div className="text-red-600 text-center py-8">{fetchState.message}</div>
          ) : (
            <>
              <div>
                <label className="block text-xs font-semibold uppercase tracking-wider text-gray-500 mb-1">
                  Model
                </label>
                <select
                  value={selectedModel}
                  onChange={(e) => setSelectedModel(e.target.value)}
                  className="w-full rounded-md border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                >
                  {allModelOptions.length === 0 && <option value="">No models available</option>}
                  {models.local.length > 0 && (
                    <optgroup label="Local">
                      {models.local.map((m) => (
                        <option key={m.id} value={m.id}>
                          {m.displayName ?? m.id}
                        </option>
                      ))}
                    </optgroup>
                  )}
                  {models.remote.length > 0 && (
                    <optgroup label="Remote">
                      {models.remote.map((m) => (
                        <option key={m.id} value={m.id}>
                          {m.displayName ?? m.id}
                        </option>
                      ))}
                    </optgroup>
                  )}
                </select>
                {allModelOptions.find((m) => m.model.id === selectedModel)?.model.requiresApiKey && (
                  <p className="mt-1 text-[11px] text-yellow-700">This model requires an API key.</p>
                )}
              </div>

              <div>
                <label className="block text-xs font-semibold uppercase tracking-wider text-gray-500 mb-1">
                  Scenarios
                </label>
                <div className="border border-gray-200 rounded-md p-2 space-y-2 max-h-[40vh] overflow-y-auto bg-gray-50">
                  {Object.entries(scenariosByCategory).length === 0 ? (
                    <div className="text-xs text-gray-500 text-center py-4">No scenarios available</div>
                  ) : (
                    Object.entries(scenariosByCategory).map(([category, items]) => (
                      <ScenarioGroup
                        key={category}
                        category={category}
                        items={items}
                        selectedIds={selectedIds}
                        onToggle={toggleScenario}
                        onSelectAll={selectAll}
                        onClear={clearGroup}
                      />
                    ))
                  )}
                </div>
                <div className="mt-1 text-xs text-gray-500">
                  {selectedIds.size} scenario{selectedIds.size === 1 ? "" : "s"} selected
                </div>
              </div>

              <div>
                <button
                  type="button"
                  onClick={() => setShowAdvanced((prev) => !prev)}
                  className="flex items-center gap-1 text-xs font-semibold uppercase tracking-wider text-gray-500 hover:text-gray-700"
                >
                  {showAdvanced ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                  Advanced Options
                </button>
                {showAdvanced && (
                  <div className="mt-2 space-y-3 border-l-2 border-gray-200 pl-3">
                    <div>
                      <label className="block text-xs font-medium text-gray-500 mb-1">
                        System Prompt
                      </label>
                      <textarea
                        value={systemPrompt}
                        onChange={(e) => setSystemPrompt(e.target.value)}
                        rows={3}
                        className="w-full rounded-md border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                        placeholder="Optional custom system prompt"
                      />
                    </div>
                    <div>
                      <label className="block text-xs font-medium text-gray-500 mb-1">
                        Timeout (seconds)
                      </label>
                      <input
                        type="number"
                        value={timeoutSecs}
                        onChange={(e) => setTimeoutSecs(Number(e.target.value))}
                        min={1}
                        className="w-full rounded-md border border-gray-200 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
                      />
                    </div>
                  </div>
                )}
              </div>
            </>
          )}

          {submitError && (
            <div className="text-red-600 text-xs border border-red-200 px-3 py-2 bg-red-50 rounded-sm">
              {submitError}
            </div>
          )}
        </div>

        <div className="flex justify-end gap-3 px-4 py-3 border-t border-gray-200">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-1.5 text-xs border border-gray-200 text-gray-600 hover:bg-gray-50 rounded-md"
          >
            Cancel
          </button>
          <button
            type="submit"
            autoFocus
            disabled={isSubmitting || fetchState.state !== "ready" || selectedIds.size === 0}
            className="px-4 py-1.5 text-xs border border-blue-600 text-blue-700 hover:bg-blue-50 disabled:opacity-50 disabled:cursor-not-allowed rounded-md"
          >
            {isSubmitting ? "Starting…" : "Start Run"}
          </button>
        </div>
      </form>
    </dialog>
  );
}

function ScenarioGroup({
  category,
  items,
  selectedIds,
  onToggle,
  onSelectAll,
  onClear,
}: {
  category: string;
  items: ScenarioInfo[];
  selectedIds: Set<string>;
  onToggle: (id: string) => void;
  onSelectAll: (ids: string[]) => void;
  onClear: (ids: string[]) => void;
}) {
  const ids = items.map((i) => i.id);

  return (
    <div className="border border-gray-200 rounded-md p-2 bg-white">
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs font-semibold uppercase tracking-wider text-gray-700">{category}</span>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => onSelectAll(ids)}
            className="text-[11px] text-blue-600 hover:underline"
          >
            all
          </button>
          <button
            type="button"
            onClick={() => onClear(ids)}
            className="text-[11px] text-gray-500 hover:underline"
          >
            clear
          </button>
        </div>
      </div>
      <div className="space-y-1">
        {items.map((s) => (
          <label
            key={s.id}
            className="flex items-center gap-2 text-xs text-gray-700 hover:bg-gray-50 rounded px-1 cursor-pointer"
          >
            <input
              type="checkbox"
              checked={selectedIds.has(s.id)}
              onChange={() => onToggle(s.id)}
              className="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
            />
            <span className="font-medium">{s.id}</span>
            <span className="text-gray-500 truncate">{s.name}</span>
            <span className="ml-auto text-[10px] text-gray-400">{s.maxPoints}pt</span>
          </label>
        ))}
      </div>
    </div>
  );
}
