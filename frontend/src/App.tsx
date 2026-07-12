import { useEffect, useState } from "react";
import { LayoutDashboard, History, FlaskConical } from "lucide-react";
import { Dashboard } from "./views/Dashboard";
import { StartRunModal } from "./components/StartRunModal";
import { ToastProvider } from "./components/Toaster";

type View = { name: "dashboard" } | { name: "history" } | { name: "oneshot" };

function parseView(): View {
  const params = new URLSearchParams(window.location.search);
  const name = params.get("view") ?? "dashboard";
  if (name === "history" || name === "oneshot") return { name };
  return { name: "dashboard" };
}

function setView(view: View) {
  const params = new URLSearchParams(window.location.search);
  params.set("view", view.name);
  window.history.pushState(null, "", `?${params.toString()}`);
}

export default function App() {
  const [view, setViewState] = useState<View>(parseView);
  const [isRunModalOpen, setIsRunModalOpen] = useState(false);

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
            </nav>
          </div>
        </header>

        <main className="mx-auto max-w-7xl px-6 py-6">
          {view.name === "dashboard" && (
            <Dashboard
              onStartRun={() => setIsRunModalOpen(true)}
              onHistory={() => navigate({ name: "history" })}
            />
          )}
          {view.name === "history" && <RunHistory onBack={() => navigate({ name: "dashboard" })} />}
          {view.name === "oneshot" && <OneShotLab onBack={() => navigate({ name: "dashboard" })} />}
        </main>
      </div>

      {isRunModalOpen && (
        <StartRunModal
          onClose={() => setIsRunModalOpen(false)}
          onLaunch={() => setIsRunModalOpen(false)}
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

function RunHistory({ onBack }: { onBack: () => void }) {
  return (
    <div className="space-y-6">
      <div className="rounded-xl border bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">Run History</h2>
          <button onClick={onBack} className="text-sm text-blue-600 hover:underline">
            Back to Dashboard
          </button>
        </div>
        <p className="mt-2 text-gray-600">Leaderboard and past runs will appear here.</p>
      </div>
    </div>
  );
}

function OneShotLab({ onBack }: { onBack: () => void }) {
  return (
    <div className="space-y-6">
      <div className="rounded-xl border bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">One-shot Lab</h2>
          <button onClick={onBack} className="text-sm text-blue-600 hover:underline">
            Back to Dashboard
          </button>
        </div>
        <p className="mt-2 text-gray-600">Single-prompt model tests will appear here.</p>
      </div>
    </div>
  );
}
