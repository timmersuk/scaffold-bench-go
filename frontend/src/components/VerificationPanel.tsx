import type { ScenarioState, EvaluationCheck } from "../types";
import { Panel } from "./Panel";

interface CategoryRollup {
  category: string;
  points: number;
  maxPoints: number;
}

interface VerificationPanelProps {
  scenario?: ScenarioState;
  isRunComplete: boolean;
  categoryRollups?: CategoryRollup[];
  totalPoints?: number;
  maxPoints?: number;
}

function Check({ pass, label, detail }: { pass: boolean; label: string; detail?: string }) {
  return (
    <div className="flex gap-2 items-start">
      <span className={`flex-shrink-0 mt-0.5 text-[13px] ${pass ? "text-green-600" : "text-gray-400"}`}>
        {pass ? "✓" : "·"}
      </span>
      <div className="flex flex-col min-w-0">
        <span className={`text-xs ${pass ? "text-gray-900" : "text-gray-500"}`}>{label}</span>
        {detail && <span className="text-[11px] text-gray-500 mt-0.5 break-words">{detail}</span>}
      </div>
    </div>
  );
}

function EvalCheck({ check }: { check: EvaluationCheck }) {
  return (
    <div className="flex gap-2 items-start">
      <span className={`flex-shrink-0 mt-0.5 text-[13px] ${check.pass ? "text-green-600" : "text-red-600"}`}>
        {check.pass ? "✓" : "✗"}
      </span>
      <div className="flex flex-col min-w-0">
        <span className="text-xs text-gray-900">{check.name}</span>
        {check.detail && (
          <span className="text-[11px] text-gray-500 mt-0.5 break-words border-l-2 border-red-200 pl-2">
            {check.detail}
          </span>
        )}
      </div>
    </div>
  );
}

export function VerificationPanel({
  scenario,
  isRunComplete,
  categoryRollups,
  totalPoints,
  maxPoints,
}: VerificationPanelProps) {
  const hasAgentOutput =
    (scenario?.logs.length ?? 0) > 0 || (scenario?.streamBuffer.trim().length ?? 0) > 0;
  const toolActivityCount = scenario?.toolCallCount ?? 0;
  const bashCalls = scenario?.bashCallCount ?? 0;
  const hasVerificationCommands = bashCalls > 0;
  const hasEval = !!scenario?.evaluation;

  return (
    <Panel title="Verification" className="flex-1 min-h-0">
      <div className="flex-1 overflow-y-auto p-3 bg-gray-50 flex flex-col gap-2 text-xs">
        {isRunComplete && categoryRollups && categoryRollups.length > 0 ? (
          <>
            <div className="text-[11px] text-gray-500 uppercase tracking-widest mb-1">
              Final Summary
            </div>
            {totalPoints !== undefined && maxPoints !== undefined && (
              <div className="text-sm font-bold text-gray-900 mb-2">
                {totalPoints} <span className="text-gray-500 font-normal">/ {maxPoints} pts</span>
              </div>
            )}
            <div className="border-t border-gray-200 pt-2">
              <div className="text-[11px] text-gray-500 uppercase tracking-widest mb-1">By Category</div>
              {categoryRollups.map((cat) => (
                <div key={cat.category} className="flex justify-between items-center py-0.5">
                  <span className="text-gray-500 text-[11px] uppercase">{cat.category}</span>
                  <span
                    className={`text-[11px] font-bold ${
                      cat.points === cat.maxPoints
                        ? "text-green-600"
                        : cat.points > 0
                          ? "text-yellow-600"
                          : "text-gray-400"
                    }`}
                  >
                    {cat.points}/{cat.maxPoints}
                  </span>
                </div>
              ))}
            </div>
          </>
        ) : scenario ? (
          <>
            {totalPoints !== undefined && maxPoints !== undefined && (
              <div className="text-sm font-bold text-gray-900">
                live score: {totalPoints} <span className="text-gray-500 font-normal">/ {maxPoints} pts</span>
              </div>
            )}
            <div
              className="text-[10px] text-gray-500 border border-gray-200 bg-white px-2 py-1 rounded-sm"
              title="These are live runtime signals. Final correctness is determined by Evaluation checks once the scenario finishes."
            >
              live checks = runtime signals; final correctness = evaluation checks
            </div>
            <Check pass={scenario.status !== "pending"} label="Scenario started event received" />
            <Check pass={hasAgentOutput} label="Agent output observed" />
            <Check
              pass={toolActivityCount > 0}
              label={`Tool calls observed${toolActivityCount > 0 ? ` (${toolActivityCount})` : ""}`}
            />
            <Check
              pass={hasVerificationCommands}
              label={`Bash verification commands observed${bashCalls > 0 ? ` (${bashCalls})` : ""}`}
            />
            {hasEval && scenario.evaluation && (
              <>
                <div className="border-t border-gray-200 my-1 pt-2">
                  <div className="text-[11px] text-gray-500 uppercase tracking-widest mb-2">Evaluation</div>
                  {scenario.evaluation.checks?.map((check) => (
                    <EvalCheck key={check.name} check={check} />
                  ))}
                  {scenario.evaluation.summary && (
                    <div className="mt-2 text-[11px] text-gray-500 border-l-2 border-gray-200 pl-2">
                      {scenario.evaluation.summary}
                    </div>
                  )}
                </div>
              </>
            )}
          </>
        ) : (
          <div className="text-gray-500 text-center py-4">No active scenario</div>
        )}
      </div>
    </Panel>
  );
}
