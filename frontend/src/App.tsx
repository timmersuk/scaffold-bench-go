import { useEffect, useState } from "react";
import { LayoutDashboard, FlaskConical, Settings as SettingsIcon, Trophy, Layers, History } from "lucide-react";
import { Dashboard } from "./views/Dashboard";
import { RunDetailView } from "./views/RunDetailView";
import { OneShotLab } from "./views/OneShotLab";
import { Report } from "./views/Report";
import { Settings } from "./views/Settings";
import { Batches } from "./views/Batches";
import { RunHistory } from "./views/RunHistory";
import { StartRunModal } from "./components/StartRunModal";
import { ToastProvider } from "./components/Toaster";

type View =
  | { name: "dashboard" }
  | { name: "report" }
  | { name: "oneshot" }
  | { name: "batches" }
  | { name: "history" }
  | { name: "settings" }
  | { name: "run"; runId: string }
  | { name: "batch"; batchId: string };

function parseView(): View {
  const params = new URLSearchParams(window.location.search);
  const name = params.get("view") ?? "dashboard";
  if (name === "report") return { name: "report" };
  if (name === "oneshot") return { name: "oneshot" };
  if (name === "batches") return { name: "batches" };
  if (name === "history") return { name: "history" };
  if (name === "settings") return { name: "settings" };
  if (name === "run") {
    const runId = params.get("runId");
    if (runId) return { name: "run", runId };
  }
  if (name === "batch") {
    const batchId = params.get("batchId");
    if (batchId) return { name: "batch", batchId };
  }
  return { name: "dashboard" };
}

function setView(view: View) {
  const params = new URLSearchParams(window.location.search);
  params.set("view", view.name);
  if (view.name === "run") {
    params.set("runId", view.runId);
    params.delete("batchId");
  } else if (view.name === "batch") {
    params.set("batchId", view.batchId);
    params.delete("runId");
  } else {
    params.delete("runId");
    params.delete("batchId");
  }
  window.history.pushState(null, "", `?${params.toString()}`);
}

export default function App() {
  const [view, setViewState] = useState<View>(parseView);
  const [isRunModalOpen, setIsRunModalOpen] = useState(false);
  const [startingRunId, setStartingRunId] = useState<string | null>(null);

  useEffect(() => {
    const onPop = () => setViewState(parseView());
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  const navigate = (next: View) => {
    setView(next);
    setViewState(next);
  };

  const link = (next: View) => ({
    onClick: () => navigate(next),
    active: view.name === next.name,
  });

  return (
    <ToastProvider>
      <div className="h-screen flex flex-col bg-gray-50 text-gray-900 overflow-hidden">
        <header className="flex-none border-b bg-white px-6 py-4">
          <div className="mx-auto flex max-w-7xl items-center justify-between">
            <h1 className="text-xl font-semibold tracking-tight">Scaffold Bench</h1>
            <nav className="flex gap-2">
              <NavButton {...link({ name: "dashboard" })} icon={<LayoutDashboard size={18} />}>
                Dashboard
              </NavButton>
              <NavButton {...link({ name: "report" })} icon={<Trophy size={18} />}>
                Leaderboard
              </NavButton>
              <NavButton {...link({ name: "batches" })} icon={<Layers size={18} />}>
                Batches
              </NavButton>
              <NavButton {...link({ name: "history" })} icon={<History size={18} />}>
                History
              </NavButton>
              <NavButton {...link({ name: "oneshot" })} icon={<FlaskConical size={18} />}>
                One-shot
              </NavButton>
              <NavButton {...link({ name: "settings" })} icon={<SettingsIcon size={18} />}>
                Settings
              </NavButton>
            </nav>
          </div>
        </header>

        <main className="flex-1 mx-auto w-full max-w-7xl px-6 py-6 min-h-0 overflow-auto">
          {view.name === "dashboard" && (
            <Dashboard
              onStartRun={() => setIsRunModalOpen(true)}
              startingRunId={startingRunId}
              onOpenBatch={(batchId) => navigate({ name: "batch", batchId })}
            />
          )}
          {view.name === "report" && <Report onBack={() => navigate({ name: "dashboard" })} />}
          {view.name === "run" && <RunDetailView runId={view.runId} onBack={() => navigate({ name: "history" })} />}
          {view.name === "batch" && (
            <Batches
              onBack={() => navigate({ name: "batches" })}
              onOpenRun={(runId) => navigate({ name: "run", runId })}
              initialBatchId={view.batchId}
            />
          )}
          {view.name === "batches" && (
            <Batches
              onBack={() => navigate({ name: "dashboard" })}
              onOpenRun={(runId) => navigate({ name: "run", runId })}
            />
          )}
          {view.name === "history" && (
            <RunHistory
              onBack={() => navigate({ name: "dashboard" })}
              onOpenRun={(runId) => navigate({ name: "run", runId })}
            />
          )}
          {view.name === "oneshot" && <OneShotLab onBack={() => navigate({ name: "dashboard" })} />}
          {view.name === "settings" && <Settings onBack={() => navigate({ name: "dashboard" })} />}
        </main>
      </div>

      {isRunModalOpen && (
        <StartRunModal
          onClose={() => setIsRunModalOpen(false)}
          onLaunch={(runId) => {
            setStartingRunId(runId);
            setIsRunModalOpen(false);
          }}
        />
      )}
    </ToastProvider>
  );
}

function NavButton({
  active,
  onClick,
  icon,
  children,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      className={`flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors ${
        active ? "bg-gray-900 text-white" : "text-gray-600 hover:bg-gray-100"
      }`}
    >
      {icon}
      {children}
    </button>
  );
}
