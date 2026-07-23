import { useEffect, useState, useRef } from "react";
import { ArrowLeft, Play, Square, RefreshCw } from "lucide-react";
import { api } from "../api";
import type { BatchRun, BatchRunDetail, ScenarioInfo, ModelsResponse } from "../types";

interface BatchesProps {
  onBack: () => void;
  onOpenRun: (runId: string) => void;
  initialBatchId?: string;
}

type ViewState =
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "list"; batches: BatchRun[] }
  | { kind: "detail"; batchId: string; batch: BatchRunDetail };

export function Batches({ onBack, onOpenRun, initialBatchId }: BatchesProps) {
  const [state, setState] = useState<ViewState>({ kind: "loading" });
  const [showStartForm, setShowStartForm] = useState(false);
  const pollRef = useRef<number | null>(null);

  const clearPoll = () => {
    if (pollRef.current !== null) {
      window.clearInterval(pollRef.current);
      pollRef.current = null;
    }
  };

  useEffect(() => {
    if (initialBatchId) {
      loadBatchDetail(initialBatchId);
    } else {
      loadBatches();
    }
    return clearPoll;
  }, [initialBatchId]);

  // Poll batch detail when viewing a running batch
  useEffect(() => {
    if (state.kind !== "detail") {
      clearPoll();
      return;
    }

    if (state.batch.batch.status !== "running") {
      clearPoll();
      return;
    }

    clearPoll();
    pollRef.current = window.setInterval(async () => {
      try {
        const batch = await api.getBatchRun(state.batchId);
        setState({ kind: "detail", batchId: state.batchId, batch });
      } catch {
        // ignore polling errors
      }
    }, 2000);

    return clearPoll;
  }, [state.kind === "detail" ? state.batch.batch.status : null, state.kind === "detail" ? state.batchId : null]);

  // Poll batch list when there's a running batch
  useEffect(() => {
    if (state.kind !== "list") return;
    if (!state.batches.some((b) => b.status === "running")) return;

    clearPoll();
    pollRef.current = window.setInterval(async () => {
      try {
        const batches = await api.listBatchRuns();
        setState({ kind: "list", batches });
      } catch {
        // ignore polling errors
      }
    }, 3000);

    return clearPoll;
  }, [state.kind === "list" ? state.batches.map((b) => b.status).join(",") : null]);

  async function loadBatches() {
    clearPoll();
    try {
      const batches = await api.listBatchRuns();
      setState({ kind: "list", batches });
    } catch (err) {
      setState({ kind: "error", message: err instanceof Error ? err.message : "Failed to load batches" });
    }
  }

  async function loadBatchDetail(batchId: string) {
    try {
      const batch = await api.getBatchRun(batchId);
      setState({ kind: "detail", batchId, batch });
    } catch (err) {
      setState({ kind: "error", message: err instanceof Error ? err.message : "Failed to load batch" });
    }
  }

  async function stopBatch(batchId: string) {
    try {
      await api.stopBatchRun(batchId);
      await loadBatches();
    } catch (err) {
      alert(err instanceof Error ? err.message : "Failed to stop batch");
    }
  }

  if (state.kind === "loading") {
    return (
      <div className="space-y-6">
        <div className="rounded-xl border bg-white p-6 shadow-sm">
          <div className="flex items-center justify-center gap-2 text-gray-500">
            <span className="w-4 h-4 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
            Loading batches...
          </div>
        </div>
      </div>
    );
  }

  if (state.kind === "error") {
    return (
      <div className="space-y-6">
        <div className="rounded-xl border bg-white p-6 shadow-sm">
          <div className="flex items-center justify-between">
            <button
              onClick={onBack}
              className="flex items-center gap-1 text-sm text-gray-600 hover:text-gray-900"
            >
              <ArrowLeft size={16} />
              Back to Dashboard
            </button>
          </div>
          <div className="mt-8 rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700">
            {state.message}
          </div>
        </div>
      </div>
    );
  }

  if (state.kind === "detail") {
    return (
      <BatchDetailView
        batch={state.batch}
        onBack={loadBatches}
        onStop={() => stopBatch(state.batch.batch.id)}
        onOpenRun={onOpenRun}
      />
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
            <h2 className="text-lg font-semibold">Batch Runs</h2>
          </div>
          <div className="flex gap-2">
            <button
              onClick={loadBatches}
              className="flex items-center gap-2 rounded-md border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              <RefreshCw size={16} />
              Refresh
            </button>
            <button
              onClick={() => setShowStartForm(true)}
              className="flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700"
            >
              <Play size={16} />
              Start Batch
            </button>
          </div>
        </div>

        {state.batches.length === 0 ? (
          <div className="mt-8 text-center text-gray-500">
            <p>No batch runs yet.</p>
            <p className="text-sm mt-1">Start a batch run to benchmark multiple models across multiple scenarios.</p>
          </div>
        ) : (
          <div className="mt-6 space-y-3">
            {state.batches.map((batch) => (
              <BatchCard
                key={batch.id}
                batch={batch}
                onClick={() => loadBatchDetail(batch.id)}
                onStop={() => stopBatch(batch.id)}
              />
            ))}
          </div>
        )}
      </div>

      {showStartForm && (
        <StartBatchModal
          onClose={() => setShowStartForm(false)}
          onStarted={(batchId) => {
            setShowStartForm(false);
            loadBatchDetail(batchId);
          }}
        />
      )}
    </div>
  );
}

function BatchCard({ batch, onClick, onStop }: { batch: BatchRun; onClick: () => void; onStop: () => void }) {
  const statusColors = {
    running: "bg-blue-100 text-blue-700",
    completed: "bg-green-100 text-green-700",
    interrupted: "bg-yellow-100 text-yellow-700",
    failed: "bg-red-100 text-red-700",
  };

  return (
    <div className="rounded-lg border border-gray-200 p-4 hover:border-gray-300 transition-colors">
      <div className="flex items-start justify-between">
        <div className="flex-1 cursor-pointer" onClick={onClick}>
          <div className="flex items-center gap-3">
            <span className={`text-xs px-2 py-1 rounded ${statusColors[batch.status]}`}>
              {batch.status}
            </span>
            <span className="text-sm text-gray-500">
              {new Date(batch.startedAt).toLocaleString()}
            </span>
          </div>
          <div className="mt-2 text-sm text-gray-700">
            {batch.config.modelIds.length} model{batch.config.modelIds.length !== 1 ? "s" : ""} ×{" "}
            {batch.config.scenarioIds.length} scenario{batch.config.scenarioIds.length !== 1 ? "s" : ""} ×{" "}
            {batch.config.runsPerModel} run{batch.config.runsPerModel !== 1 ? "s" : ""}
          </div>
          {batch.finishedAt && (
            <div className="mt-1 text-xs text-gray-500">
              Duration: {Math.round((batch.finishedAt - batch.startedAt) / 1000)}s
            </div>
          )}
        </div>
        {batch.status === "running" && (
          <button
            onClick={onStop}
            className="flex items-center gap-1 rounded-md border border-red-300 px-3 py-1.5 text-xs font-medium text-red-700 hover:bg-red-50"
          >
            <Square size={14} />
            Stop
          </button>
        )}
      </div>
    </div>
  );
}

function BatchDetailView({ batch, onBack, onStop, onOpenRun }: {
  batch: BatchRunDetail;
  onBack: () => void;
  onStop: () => void;
  onOpenRun: (runId: string) => void;
}) {
  const statusColors = {
    running: "bg-blue-100 text-blue-700",
    completed: "bg-green-100 text-green-700",
    interrupted: "bg-yellow-100 text-yellow-700",
    failed: "bg-red-100 text-red-700",
  };

  const completedRuns = batch.runs.filter((r) => r.status === "done" || r.status === "failed" || r.status === "stopped").length;
  const totalRuns = batch.batch.config.modelIds.length * batch.batch.config.runsPerModel;
  const progressPct = totalRuns > 0 ? Math.round((completedRuns / totalRuns) * 100) : 0;

  return (
    <div className="space-y-6">
      <div className="rounded-xl border bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between">
          <button
            onClick={onBack}
            className="flex items-center gap-1 text-sm text-gray-600 hover:text-gray-900"
          >
            <ArrowLeft size={16} />
            Back to Batches
          </button>
          {batch.batch.status === "running" && (
            <button
              onClick={onStop}
              className="flex items-center gap-2 rounded-md border border-red-300 px-3 py-2 text-sm font-medium text-red-700 hover:bg-red-50"
            >
              <Square size={16} />
              Stop Batch
            </button>
          )}
        </div>

        <div className="mt-6">
          <div className="flex items-center gap-3">
            <span className={`text-xs px-2 py-1 rounded ${statusColors[batch.batch.status]}`}>
              {batch.batch.status}
            </span>
            <span className="text-sm text-gray-500">
              {new Date(batch.batch.startedAt).toLocaleString()}
            </span>
          </div>

          <div className="mt-4">
            <div className="flex items-center justify-between text-sm text-gray-700">
              <span>Progress</span>
              <span>{completedRuns} / {totalRuns} runs ({progressPct}%)</span>
            </div>
            <div className="mt-2 h-2 rounded-full bg-gray-200 overflow-hidden">
              <div
                className="h-full bg-blue-600 transition-all"
                style={{ width: `${progressPct}%` }}
              />
            </div>
          </div>

          <div className="mt-6 grid grid-cols-2 gap-4 text-sm">
            <div>
              <div className="text-gray-500">Models</div>
              <div className="mt-1">{batch.batch.config.modelIds.length}</div>
            </div>
            <div>
              <div className="text-gray-500">Scenarios</div>
              <div className="mt-1">{batch.batch.config.scenarioIds.length}</div>
            </div>
            <div>
              <div className="text-gray-500">Runs per Model</div>
              <div className="mt-1">{batch.batch.config.runsPerModel}</div>
            </div>
            <div>
              <div className="text-gray-500">Warmup Duration</div>
              <div className="mt-1">{batch.batch.config.warmupDuration}s</div>
            </div>
            <div>
              <div className="text-gray-500">Harness</div>
              <div className="mt-1">{batch.batch.config.harness || "default"}</div>
            </div>
            {batch.batch.finishedAt && (
              <div>
                <div className="text-gray-500">Duration</div>
                <div className="mt-1">{Math.round((batch.batch.finishedAt - batch.batch.startedAt) / 1000)}s</div>
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="rounded-xl border bg-white p-6 shadow-sm">
        <h3 className="text-lg font-semibold">Runs</h3>
        {batch.runs.length === 0 ? (
          <div className="mt-4 text-center text-gray-500">
            <p>No runs started yet.</p>
          </div>
        ) : (
          <div className="mt-4 space-y-2">
            {batch.runs.map((run) => (
              <div
                key={run.id}
                className="flex items-center justify-between rounded-lg border border-gray-200 p-3 hover:border-gray-300 transition-colors cursor-pointer"
                onClick={() => onOpenRun(run.id)}
              >
                <div>
                  <div className="flex items-center gap-2">
                    <span className={`text-xs px-2 py-1 rounded ${
                      run.status === "done" ? "bg-green-100 text-green-700" :
                      run.status === "failed" ? "bg-red-100 text-red-700" :
                      run.status === "stopped" ? "bg-yellow-100 text-yellow-700" :
                      "bg-blue-100 text-blue-700"
                    }`}>
                      {run.status}
                    </span>
                    <span className="text-sm font-medium">{run.model}</span>
                  </div>
                  <div className="mt-1 text-xs text-gray-500">
                    {new Date(run.startedAt).toLocaleString()}
                  </div>
                </div>
                <div className="text-right">
                  {run.totalPoints !== null && run.maxPoints !== null && (
                    <div className="text-sm font-medium">
                      {run.totalPoints} / {run.maxPoints}
                    </div>
                  )}
                  <div className="text-xs text-gray-500">
                    {new Date(run.startedAt).toLocaleDateString()}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function StartBatchModal({ onClose, onStarted }: { onClose: () => void; onStarted: (batchId: string) => void }) {
  const [scenarios, setScenarios] = useState<ScenarioInfo[]>([]);
  const [models, setModels] = useState<ModelsResponse>({ local: [], remote: [] });
  const [selectedModels, setSelectedModels] = useState<string[]>([]);
  const [selectedScenarios, setSelectedScenarios] = useState<string[]>([]);
  const [runsPerModel, setRunsPerModel] = useState(3);
  const [warmupDuration, setWarmupDuration] = useState(25);
  const [harness, setHarness] = useState("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    Promise.all([api.getScenarios(), api.getModels()])
      .then(([scenarios, models]) => {
        setScenarios(scenarios);
        setModels(models);
        setSelectedScenarios(scenarios.map((s) => s.id));
        setLoading(false);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load data");
        setLoading(false);
      });
  }, []);

  const allModels = [...models.local, ...models.remote];

  function toggleModel(modelId: string) {
    setSelectedModels((prev) =>
      prev.includes(modelId) ? prev.filter((id) => id !== modelId) : [...prev, modelId]
    );
  }

  function toggleScenario(scenarioId: string) {
    setSelectedScenarios((prev) =>
      prev.includes(scenarioId) ? prev.filter((id) => id !== scenarioId) : [...prev, scenarioId]
    );
  }

  function selectAllScenarios() {
    setSelectedScenarios(scenarios.map((s) => s.id));
  }

  function deselectAllScenarios() {
    setSelectedScenarios([]);
  }

  async function handleSubmit() {
    if (selectedModels.length === 0) {
      setError("Select at least one model");
      return;
    }
    if (selectedScenarios.length === 0) {
      setError("Select at least one scenario");
      return;
    }

    setSubmitting(true);
    setError("");

    try {
      const { batchId } = await api.createBatchRun({
        modelIds: selectedModels,
        scenarioIds: selectedScenarios,
        runsPerModel,
        warmupDuration,
        harness,
      });
      onStarted(batchId);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start batch");
      setSubmitting(false);
    }
  }

  if (loading) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
        <div className="rounded-xl bg-white p-6 shadow-xl">
          <div className="flex items-center gap-2 text-gray-500">
            <span className="w-4 h-4 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
            Loading...
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="max-h-[90vh] w-full max-w-4xl overflow-y-auto rounded-xl bg-white p-6 shadow-xl">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">Start Batch Run</h2>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600">
            ✕
          </button>
        </div>

        {error && (
          <div className="mt-4 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            {error}
          </div>
        )}

        <div className="mt-6 space-y-6">
          <div>
            <label className="block text-sm font-medium text-gray-700">Models</label>
            <div className="mt-2 max-h-48 overflow-y-auto rounded-md border border-gray-300 p-3">
              {allModels.length === 0 ? (
                <div className="text-sm text-gray-500">No models available</div>
              ) : (
                <div className="space-y-2">
                  {allModels.map((model) => (
                    <label key={model.id} className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={selectedModels.includes(model.id)}
                        onChange={() => toggleModel(model.id)}
                        className="rounded border-gray-300"
                      />
                      <span className="text-sm">{model.displayName || model.id}</span>
                      <span className="text-xs text-gray-500">({model.source})</span>
                    </label>
                  ))}
                </div>
              )}
            </div>
          </div>

          <div>
            <div className="flex items-center justify-between">
              <label className="block text-sm font-medium text-gray-700">Scenarios</label>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={selectAllScenarios}
                  className="text-xs text-blue-600 hover:text-blue-700"
                >
                  Select All
                </button>
                <button
                  type="button"
                  onClick={deselectAllScenarios}
                  className="text-xs text-blue-600 hover:text-blue-700"
                >
                  Deselect All
                </button>
              </div>
            </div>
            <div className="mt-2 max-h-48 overflow-y-auto rounded-md border border-gray-300 p-3">
              <div className="space-y-2">
                {scenarios.map((scenario) => (
                  <label key={scenario.id} className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={selectedScenarios.includes(scenario.id)}
                      onChange={() => toggleScenario(scenario.id)}
                      className="rounded border-gray-300"
                    />
                    <span className="text-sm">{scenario.id}</span>
                    <span className="text-xs text-gray-500">({scenario.category})</span>
                  </label>
                ))}
              </div>
            </div>
            <div className="mt-1 text-xs text-gray-500">
              {selectedScenarios.length} of {scenarios.length} selected
            </div>
          </div>

          <div className="grid grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700">Runs per Model</label>
              <input
                type="number"
                min="1"
                value={runsPerModel}
                onChange={(e) => setRunsPerModel(parseInt(e.target.value) || 1)}
                className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Warmup Duration (s)</label>
              <input
                type="number"
                min="0"
                value={warmupDuration}
                onChange={(e) => setWarmupDuration(parseInt(e.target.value) || 0)}
                className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Harness</label>
              <select
                value={harness}
                onChange={(e) => setHarness(e.target.value)}
                className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm"
              >
                <option value="">Default</option>
                <option value="native">Native</option>
                <option value="hermes">Hermes</option>
                <option value="qwen">Qwen</option>
              </select>
            </div>
          </div>

          <div className="flex justify-end gap-3">
            <button
              onClick={onClose}
              className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              Cancel
            </button>
            <button
              onClick={handleSubmit}
              disabled={submitting}
              className="flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
            >
              {submitting ? (
                <>
                  <span className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                  Starting...
                </>
              ) : (
                <>
                  <Play size={16} />
                  Start Batch
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
