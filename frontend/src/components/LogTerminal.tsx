import { useEffect, useRef, useState } from "react";
import type { ScenarioState, LogEntry as LogEntryType } from "../types";
import { formatElapsed, formatNowHHMMSS } from "../lib/format";
import { Panel } from "./Panel";

interface LogTerminalProps {
  scenario?: ScenarioState;
  isLive: boolean;
}

const LABEL_STYLES: Record<string, { label: string; text: string }> = {
  assistant: { label: "text-indigo-600", text: "text-gray-700 whitespace-pre-wrap" },
  reasoning: { label: "text-purple-400", text: "text-gray-500 whitespace-pre-wrap" },
  cmd: { label: "text-blue-600", text: "text-blue-700 font-semibold whitespace-pre-wrap" },
  edit: { label: "text-blue-600/70", text: "text-gray-700 whitespace-pre-wrap" },
  tool: { label: "text-blue-600", text: "text-gray-800 whitespace-pre-wrap" },
  stdout: { label: "text-green-700", text: "text-green-700 whitespace-pre-wrap" },
  stderr: {
    label: "text-red-600",
    text: "text-red-600 bg-red-50 px-1 rounded-sm whitespace-pre-wrap",
  },
  system: { label: "text-gray-500", text: "text-gray-500 whitespace-pre-wrap" },
};
const FALLBACK = { label: "text-gray-500", text: "text-gray-800 whitespace-pre-wrap" };

function LogLine({ entry }: { entry: LogEntryType }) {
  if (entry.kind === "reasoning") {
    return <ReasoningBlock text={entry.text} time={entry.time} />;
  }
  const style = LABEL_STYLES[entry.label] ?? FALLBACK;
  return (
    <div className="flex gap-2 mb-1 break-words min-w-0">
      <span className="text-gray-400 w-[60px] flex-shrink-0 text-[11px]">[{entry.time}]</span>
      <span className={`w-[72px] flex-shrink-0 text-right pr-2 text-[11px] ${style.label}`}>
        {entry.label}
      </span>
      <span className={`flex-1 min-w-0 break-words ${style.text}`}>{entry.text}</span>
    </div>
  );
}

function ReasoningBlock({ text, time }: { text: string; time: string }) {
  const [expanded, setExpanded] = useState(false);
  const preview = text.length > 80 ? text.slice(0, 80) + "..." : text;
  return (
    <div className="flex gap-2 mb-1 break-words min-w-0">
      <span className="text-gray-400 w-[60px] flex-shrink-0 text-[11px]">[{time}]</span>
      <span className="w-[72px] flex-shrink-0 text-right pr-2 text-[11px] text-purple-400">
        reasoning
      </span>
      <div className="flex-1 min-w-0">
        <button
          onClick={() => setExpanded(!expanded)}
          className="text-left text-purple-400/70 hover:text-purple-300 text-[11px] cursor-pointer"
        >
          <span className="mr-1">{expanded ? "▼" : "▶"}</span>
          {expanded ? "Reasoning" : `Reasoning…`}
        </button>
        {expanded && (
          <div className="mt-1 text-gray-500 whitespace-pre-wrap text-[11px] bg-gray-800/50 rounded px-2 py-1">
            {text}
          </div>
        )}
        {!expanded && (
          <span className="text-gray-600 text-[11px] ml-1">{preview}</span>
        )}
      </div>
    </div>
  );
}

function LiveReasoning({ text }: { text: string }) {
  return (
    <div className="flex gap-2 mb-1 min-w-0">
      <span className="text-gray-500 w-[60px] flex-shrink-0 text-[11px]">
        [{formatNowHHMMSS()}]
      </span>
      <span className="w-[72px] flex-shrink-0 text-right pr-2 text-[11px] text-purple-400">
        reasoning
      </span>
      <span className="flex-1 min-w-0 text-gray-500 whitespace-pre-wrap break-words italic">
        {text}
        <span className="inline-block w-[7px] h-[13px] bg-purple-400/50 animate-pulse translate-y-0.5 ml-0.5" />
      </span>
    </div>
  );
}

export function LogTerminal({ scenario, isLive }: LogTerminalProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const isUserScrolledUp = useRef(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isUserScrolledUp.current && scrollRef.current) {
      scrollRef.current.scrollIntoView({ behavior: "auto" });
    }
  }, [scenario?.logs.length, scenario?.streamBuffer, scenario?.reasoningBuffer]);

  const handleScroll = () => {
    const el = containerRef.current;
    if (!el) return;
    const atBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - 30;
    isUserScrolledUp.current = !atBottom;
  };

  const title = scenario
    ? scenario.name && scenario.name !== scenario.id
      ? `${scenario.id} / ${scenario.name}`
      : scenario.id
    : "Agent Log";

  const statusTag = scenario?.status === "running" ? "RUNNING" : scenario?.status?.toUpperCase();

  const elapsed = scenario?.startedAt ? (scenario.finishedAt ?? Date.now()) - scenario.startedAt : 0;

  return (
    <Panel title={title} className="h-full">
      {scenario ? (
        <div className="flex flex-col h-full bg-gray-900 text-gray-100 overflow-hidden">
          <div className="flex-none px-4 py-2 border-b border-gray-700 bg-gray-800 flex justify-between items-center text-[11px]">
            <div className="flex gap-3 items-center">
              <span className="text-gray-400">{scenario.category}</span>
              {scenario.status === "running" && (
                <span className="text-purple-400 uppercase">● RUNNING</span>
              )}
              {scenario.status !== "running" && statusTag && (
                <span
                  className={`uppercase ${
                    scenario.status === "pass"
                      ? "text-green-400"
                      : scenario.status === "fail" || scenario.status === "stopped"
                        ? "text-red-400"
                        : scenario.status === "partial"
                          ? "text-yellow-400"
                          : "text-gray-400"
                  }`}
                >
                  {statusTag}
                </span>
              )}
            </div>
            <div className="flex gap-3 text-gray-400">
              {scenario.toolCallCount !== undefined && <span>tools {scenario.toolCallCount}</span>}
              {elapsed > 0 && <span>elapsed {formatElapsed(elapsed)}</span>}
            </div>
          </div>

          <div
            ref={containerRef}
            onScroll={handleScroll}
            className="flex-1 overflow-y-auto px-4 py-3 text-xs font-mono"
          >
            {scenario.prompt && (
              <div className="flex gap-2 mb-2 pb-2 border-b border-gray-700 break-words min-w-0">
                <span className="text-gray-400 w-[60px] flex-shrink-0 text-[11px]">[--:--:--]</span>
                <span className="w-[72px] flex-shrink-0 text-right pr-2 text-[11px] text-amber-400">
                  user
                </span>
                <span className="flex-1 min-w-0 break-words text-gray-200 whitespace-pre-wrap">
                  {scenario.prompt}
                </span>
              </div>
            )}

            {scenario.logs.map((entry) => (
              <LogLine key={entry.id} entry={entry} />
            ))}

            {isLive && scenario.reasoningBuffer && (
              <LiveReasoning text={scenario.reasoningBuffer} />
            )}

            {isLive && scenario.streamBuffer && (
              <div className="flex gap-2 mb-1 min-w-0">
                <span className="text-gray-500 w-[60px] flex-shrink-0 text-[11px]">
                  [{formatNowHHMMSS()}]
                </span>
                <span className="w-[72px] flex-shrink-0 text-right pr-2 text-[11px] text-indigo-400">
                  assistant
                </span>
                <span className="flex-1 min-w-0 text-gray-300 whitespace-pre-wrap break-words">
                  {scenario.streamBuffer}
                  <span className="inline-block w-[7px] h-[13px] bg-purple-400 animate-pulse translate-y-0.5 ml-0.5" />
                </span>
              </div>
            )}

            {isLive && !scenario.streamBuffer && !scenario.reasoningBuffer && (
              <div className="flex gap-2 mt-2">
                <span className="text-gray-500 w-[60px] flex-shrink-0 text-[11px]">
                  [{formatNowHHMMSS()}]
                </span>
                <span className="w-[72px] flex-shrink-0 text-right pr-2 text-[11px] text-indigo-400">
                  assistant
                </span>
                <span className="flex-1">
                  <span className="inline-block w-[7px] h-[13px] bg-indigo-400 animate-pulse translate-y-0.5" />
                </span>
              </div>
            )}

            <div ref={scrollRef} />
          </div>
        </div>
      ) : (
        <div className="flex-1 flex items-center justify-center text-gray-400 text-sm bg-gray-900">
          waiting for a scenario to start…
        </div>
      )}
    </Panel>
  );
}
