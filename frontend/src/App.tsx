import { useEffect, useState } from "react";
import { LayoutDashboard, History, FlaskConical, Settings as SettingsIcon } from "lucide-react";
import { Dashboard } from "./views/Dashboard";
import { RunHistory } from "./views/RunHistory";
import { RunDetailView } from "./views/RunDetailView";
import { OneShotLab } from "./views/OneShotLab";
import { Settings } from "./views/Settings";
import { StartRunModal } from "./components/StartRunModal";
import { ToastProvider } from "./components/Toaster";

type View =
  | { name: "dashboard" }
  | { name: "history" }
  | { name: "oneshot" }
  | { name: "settings" }
  | { name: "run"; runId: string };

function parseView(): View {
  const params = new URLSearchParams(window.location.search);
  const name = params.get("view") ?? "dashboard";
  if (name === "history") return { name: "history" };
  if (name === "oneshot") return { name: "oneshot" };
  if (name === "settings") return { name: "settings" };
  if (name === "run") {
    const runId = params.get("runId");
    if (runId) return { name: "run", runId };
  }
  return { name: "dashboard" };
}

function setView(view: View) {
  const params = new URLSearchParams(window.location.search);
  params.set("view", view.name);
  if (view.name === "run") {
    params.set("runId", view.runId);
  } else {
    params.delete("runId");
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
      <div className="min-h-screen bg-gray-50 text-gray-900">
        <header className="border-b bg-white px-6 py-4">
          <div className="mx-auto flex max-w-7xl items-center justify-between">
            <h1 className="text-xl font-semibold tracking-tight">Scaffold Bench</h1>
            <nav className="flex gap-2">
              <NavButton {...link({ name: "dashboard" })} icon={<LayoutDashboard size={18} />}>
                Dashboard
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

        <main className="mx-auto max-w-7xl px-6 py-6">
          {view.name === "dashboard" && (
            <Dashboard
              onStartRun={() => setIsRunModalOpen(true)}
              onHistory={() => navigate({ name: "history" })}
              startingRunId={startingRunId}
            />
          )}
          {view.name === "history" && (
            <RunHistory
              onBack={() => navigate({ name: "dashboard" })}
              onOpenRun={(runId) => navigate({ name: "run", runId })}
            />
          )}
          {view.name === "run" && <RunDetailView runId={view.runId} onBack={() => navigate({ name: "history" })} />}
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
