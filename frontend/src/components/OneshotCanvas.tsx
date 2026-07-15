import { useState } from "react";
import { Download, Copy, ExternalLink, RefreshCw } from "lucide-react";
import type { OneshotPromptState } from "../hooks/oneshot-state-reducer";
import { extractHtml } from "../lib/extract-html";

type Props = {
  promptState: OneshotPromptState | null;
  promptId: string | null;
  promptText?: string;
};

export function OneshotCanvas({ promptState, promptId, promptText }: Props) {
  const [viewMode, setViewMode] = useState<"artifact" | "raw" | "prompt">("artifact");

  if (!promptState || !promptId) {
    return (
      <div className="flex h-full items-center justify-center rounded-lg border bg-gray-50 text-gray-400">
        Select a prompt to view its output
      </div>
    );
  }

  const { output, status, artifact } = promptState;
  const html = extractHtml(output);
  const hasArtifact = artifact || html !== null;

  const handleDownload = () => {
    const content = html ?? output;
    const blob = new Blob([content], { type: "text/html" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${promptId}.html`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleCopy = async () => {
    await navigator.clipboard.writeText(output);
  };

  const handleOpenExternal = () => {
    const content = html ?? output;
    const blob = new Blob([content], { type: "text/html" });
    const url = URL.createObjectURL(blob);
    window.open(url, "_blank");
  };

  return (
    <div className="flex h-full flex-col rounded-lg border bg-white">
      <div className="flex items-center justify-between border-b px-4 py-2">
        <div className="flex gap-2">
          <button
            onClick={() => setViewMode("artifact")}
            className={`rounded px-3 py-1 text-xs font-medium ${
              viewMode === "artifact" ? "bg-blue-100 text-blue-700" : "text-gray-600 hover:bg-gray-100"
            }`}
          >
            Artifact
          </button>
          <button
            onClick={() => setViewMode("raw")}
            className={`rounded px-3 py-1 text-xs font-medium ${
              viewMode === "raw" ? "bg-blue-100 text-blue-700" : "text-gray-600 hover:bg-gray-100"
            }`}
          >
            Raw
          </button>
          <button
            onClick={() => setViewMode("prompt")}
            className={`rounded px-3 py-1 text-xs font-medium ${
              viewMode === "prompt" ? "bg-blue-100 text-blue-700" : "text-gray-600 hover:bg-gray-100"
            }`}
          >
            Prompt
          </button>
        </div>
        <div className="flex gap-1">
          <button onClick={handleCopy} className="rounded p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600" title="Copy output">
            <Copy size={14} />
          </button>
          <button onClick={handleDownload} className="rounded p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600" title="Download HTML">
            <Download size={14} />
          </button>
          <button onClick={handleOpenExternal} className="rounded p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600" title="Open in new tab">
            <ExternalLink size={14} />
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-hidden">
        {viewMode === "prompt" ? (
          <div className="h-full overflow-auto p-4">
            {promptText ? (
              <pre className="whitespace-pre-wrap text-sm font-mono text-gray-800">
                {promptText}
              </pre>
            ) : (
              <div className="flex h-full items-center justify-center text-gray-400">
                No prompt available
              </div>
            )}
          </div>
        ) : viewMode === "artifact" && hasArtifact ? (
          html ? (
            <div className="relative h-full">
              <iframe
                srcDoc={html}
                className="h-full w-full border-0"
                sandbox="allow-scripts"
                title={`Artifact for ${promptId}`}
              />
              {status === "running" && (
                <div className="absolute bottom-2 right-2 flex items-center gap-1.5 rounded bg-blue-500/90 px-2 py-1 text-xs text-white">
                  <RefreshCw className="animate-spin" size={12} />
                  Generating...
                </div>
              )}
            </div>
          ) : (
            <div className="flex h-full items-center justify-center text-gray-400">
              No artifact generated
            </div>
          )
        ) : (
          <pre className="h-full overflow-auto p-4 text-xs font-mono whitespace-pre-wrap">
            {output || "No output yet"}
          </pre>
        )}
      </div>
    </div>
  );
}
