import type { OneshotPromptState } from "../hooks/oneshot-state-reducer";

type Props = {
  promptState: OneshotPromptState | null;
  promptId: string | null;
  model: string | null;
};

export function OneshotMetadata({ promptState, promptId, model }: Props) {
  if (!promptState || !promptId) {
    return (
      <div className="rounded-lg border bg-white p-4 text-sm text-gray-400">
        No prompt selected
      </div>
    );
  }

  const { finishReason, wallTimeMs, firstTokenMs, promptTokens, completionTokens, error } = promptState;
  const outputRate = completionTokens && wallTimeMs ? ((completionTokens / wallTimeMs) * 1000).toFixed(1) : null;

  return (
    <div className="rounded-lg border bg-white p-4">
      <h3 className="mb-3 text-xs font-semibold uppercase tracking-wide text-gray-500">Metadata</h3>
      <dl className="space-y-2 text-sm">
        <Row label="Model" value={model ?? "—"} />
        <Row label="Prompt" value={promptId} mono />
        {finishReason && <Row label="Finish" value={finishReason} />}
        {wallTimeMs !== undefined && <Row label="Wall" value={`${wallTimeMs.toLocaleString()} ms`} />}
        {firstTokenMs !== undefined && <Row label="First token" value={`${firstTokenMs.toLocaleString()} ms`} />}
        {promptTokens !== undefined && <Row label="Prompt tokens" value={promptTokens.toLocaleString()} />}
        {completionTokens !== undefined && <Row label="Completion tokens" value={completionTokens.toLocaleString()} />}
        {outputRate && <Row label="Output rate" value={`${outputRate} tok/s`} />}
        {error && <Row label="Error" value={error} error />}
      </dl>
    </div>
  );
}

function Row({ label, value, mono, error }: { label: string; value: string; mono?: boolean; error?: boolean }) {
  return (
    <div className="flex justify-between gap-4">
      <dt className="text-gray-500">{label}</dt>
      <dd className={`text-right ${mono ? "font-mono text-xs" : ""} ${error ? "text-red-600" : "text-gray-900"}`}>
        {value}
      </dd>
    </div>
  );
}
