import type { ModelsResponse, OneshotTestSummary } from "../types";

type Props = {
  models: ModelsResponse;
  selectedModel: string;
  onModelChange: (modelId: string) => void;
  prompts: OneshotTestSummary[];
  isRunning: boolean;
  onRunAll: () => void;
  onStop: () => void;
};

export function OneshotControls({
  models,
  selectedModel,
  onModelChange,
  prompts,
  isRunning,
  onRunAll,
  onStop,
}: Props) {
  const allModels = [...models.local, ...models.remote];

  return (
    <div className="flex items-center gap-3">
      <select
        value={selectedModel}
        onChange={(e) => onModelChange(e.target.value)}
        disabled={isRunning}
        className="flex-1 rounded-md border border-gray-300 bg-white px-3 py-2 text-sm focus:border-blue-500 focus:outline-none disabled:opacity-50"
      >
        <option value="">Select a model...</option>
        {allModels.map((m) => (
          <option key={`${m.source}-${m.id}`} value={m.id}>
            {m.displayName ?? m.id} ({m.source})
          </option>
        ))}
      </select>
      {isRunning ? (
        <button
          onClick={onStop}
          className="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700"
        >
          Stop
        </button>
      ) : (
        <button
          onClick={onRunAll}
          disabled={!selectedModel || prompts.length === 0}
          className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          Run All ({prompts.length})
        </button>
      )}
    </div>
  );
}
