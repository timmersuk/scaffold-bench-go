import { useCallback, useEffect, useReducer, useState } from "react";
import { api } from "../api";
import type { ModelsResponse, OneshotTestSummary } from "../types";
import { OneshotControls } from "../components/OneshotControls";
import { OneshotQueue } from "../components/OneshotQueue";
import { OneshotCanvas } from "../components/OneshotCanvas";
import { OneshotMetadata } from "../components/OneshotMetadata";
import {
  oneshotStateReducer,
  initialState,
  hydrateFromLatestRun,
} from "../hooks/oneshot-state-reducer";
import { useOneshotSSE } from "../hooks/useOneshotSSE";

type Props = {
  onBack: () => void;
};

export function OneShotLab({ onBack }: Props) {
  const [state, dispatch] = useReducer(oneshotStateReducer, initialState);
  const [prompts, setPrompts] = useState<OneshotTestSummary[]>([]);
  const [models, setModels] = useState<ModelsResponse>({ local: [], remote: [] });
  const [selectedModel, setSelectedModel] = useState("");
  const [selectedPromptId, setSelectedPromptId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const load = async () => {
      try {
        const [tests, modelsRes, latest] = await Promise.all([
          api.oneshotTests(),
          api.getModels(),
          api.latestOneshot(),
        ]);
        setPrompts(tests);
        setModels(modelsRes);

        if (latest) {
          const hydrated = hydrateFromLatestRun(
            latest.runId,
            latest.status,
            latest.model,
            latest.promptIds,
            latest.results
          );
          dispatch({ type: "hydrate", state: hydrated });
          if (latest.promptIds.length > 0) {
            setSelectedPromptId(latest.promptIds[0]);
          }
        } else if (tests.length > 0) {
          setSelectedPromptId(tests[0].id);
        }

        if (modelsRes.local.length > 0) {
          setSelectedModel(modelsRes.local[0].id);
        } else if (modelsRes.remote.length > 0) {
          setSelectedModel(modelsRes.remote[0].id);
        }
      } catch (err) {
        console.error("Failed to load oneshot data:", err);
      } finally {
        setLoading(false);
      }
    };
    load();
  }, []);

  const handleEvent = useCallback((event: any) => {
    dispatch({ type: "event", event });
  }, []);

  useOneshotSSE(state.status === "running" || state.status === "warming_up" ? state.runId : null, handleEvent);

  const handleRunAll = async () => {
    if (!selectedModel || prompts.length === 0) return;
    try {
      const { runId } = await api.startOneshot({
        modelId: selectedModel,
        promptIds: prompts.map((p) => p.id),
      });
      dispatch({
        type: "start",
        runId,
        model: selectedModel,
        promptIds: prompts.map((p) => p.id),
      });
    } catch (err) {
      console.error("Failed to start oneshot run:", err);
    }
  };

  const handleRunSingle = async (promptId: string) => {
    if (!selectedModel) return;
    try {
      const { runId } = await api.startOneshot({
        modelId: selectedModel,
        promptIds: [promptId],
      });
      dispatch({
        type: "start",
        runId,
        model: selectedModel,
        promptIds: [promptId],
      });
    } catch (err) {
      console.error("Failed to start single prompt:", err);
    }
  };

  const handleStop = async () => {
    if (!state.runId) return;
    try {
      await api.stopOneshot(state.runId);
      dispatch({ type: "stop" });
    } catch (err) {
      console.error("Failed to stop oneshot run:", err);
    }
  };

  const isRunning = state.status === "running" || state.status === "warming_up";
  const isWarmingUp = state.status === "warming_up";
  const currentPromptState = selectedPromptId ? state.prompts[selectedPromptId] : null;
  const currentPromptText = selectedPromptId ? prompts.find(p => p.id === selectedPromptId)?.prompt : undefined;

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center text-gray-400">
        Loading...
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">ONE-SHOT LAB</h2>
          <p className="text-sm text-gray-500">UNSCORED · VIBE CHECK</p>
        </div>
        <div className="flex items-center gap-4">
          {isWarmingUp && (
            <div className="flex items-center gap-2 text-sm text-blue-600">
              <div className="h-2 w-2 animate-pulse rounded-full bg-blue-600"></div>
              <span>Warming up model...</span>
            </div>
          )}
          <button onClick={onBack} className="text-sm text-blue-600 hover:underline">
            Back to Dashboard
          </button>
        </div>
      </div>

      <OneshotControls
        models={models}
        selectedModel={selectedModel}
        onModelChange={setSelectedModel}
        prompts={prompts}
        isRunning={isRunning}
        onRunAll={handleRunAll}
        onStop={handleStop}
      />

      <div className="grid grid-cols-12 gap-6">
        <div className="col-span-4 space-y-4">
          <div className="rounded-lg border bg-white p-4">
            <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-gray-500">
              Test Queue
            </h3>
            <OneshotQueue
              prompts={prompts}
              promptStates={state.prompts}
              selectedPromptId={selectedPromptId}
              onSelect={setSelectedPromptId}
              onRunSingle={handleRunSingle}
              isRunning={isRunning}
            />
          </div>
          <OneshotMetadata
            promptState={currentPromptState}
            promptId={selectedPromptId}
            model={state.model}
          />
        </div>

        <div className="col-span-8">
          <OneshotCanvas
            promptState={currentPromptState}
            promptId={selectedPromptId}
            promptText={currentPromptText}
          />
        </div>
      </div>
    </div>
  );
}
