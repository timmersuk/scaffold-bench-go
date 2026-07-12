import React, { createContext, useContext, useState, useCallback } from "react";

export type Toast = {
  id: string;
  message: string;
  variant: "info" | "error" | "success";
};

type ToastContextValue = {
  toasts: Toast[];
  pushToast: (message: string, variant?: Toast["variant"]) => void;
  removeToast: (id: string) => void;
};

const ToastContext = createContext<ToastContextValue>({
  toasts: [],
  pushToast: () => {},
  removeToast: () => {},
});

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const pushToast = useCallback((message: string, variant: Toast["variant"] = "info") => {
    const id = `${Date.now()}-${Math.random()}`;
    setToasts((prev) => [...prev, { id, message, variant }]);
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, 5000);
  }, []);

  const removeToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ toasts, pushToast, removeToast }}>
      {children}
    </ToastContext.Provider>
  );
}

export function useToast() {
  return useContext(ToastContext);
}

export function Toaster() {
  const { toasts, removeToast } = useToast();
  if (toasts.length === 0) return null;
  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className={`max-w-sm rounded-lg border px-4 py-2 text-sm shadow-lg ${
            toast.variant === "error"
              ? "border-red-200 bg-red-50 text-red-900"
              : toast.variant === "success"
                ? "border-green-200 bg-green-50 text-green-900"
                : "border-gray-200 bg-white text-gray-900"
          }`}
        >
          <div className="flex items-center justify-between gap-4">
            <span>{toast.message}</span>
            <button
              onClick={() => removeToast(toast.id)}
              className="text-gray-400 hover:text-gray-600"
              aria-label="Dismiss"
            >
              ×
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
