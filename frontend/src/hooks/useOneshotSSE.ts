import { useEffect, useRef } from "react";
import type { BackendEvent } from "../types";

export function useOneshotSSE(
  runId: string | null,
  onEvent: (event: BackendEvent) => void
) {
  const esRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!runId) return;

    const open = () => {
      if (esRef.current) {
        esRef.current.close();
      }

      const url = `/api/oneshot/runs/${runId}/stream`;
      const es = new EventSource(url);
      esRef.current = es;

      es.onmessage = (e) => {
        try {
          const event = JSON.parse(e.data) as BackendEvent;
          onEvent(event);
        } catch {
          // ignore malformed events
        }
      };

      es.onerror = () => {
        es.close();
        esRef.current = null;
      };
    };

    const handleVisibilityChange = () => {
      if (document.hidden) return;
      
      // Tab became visible - reconnect if needed
      if (!esRef.current || esRef.current.readyState === EventSource.CLOSED) {
        open();
      }
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    open();

    return () => {
      esRef.current?.close();
      esRef.current = null;
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [runId, onEvent]);
}
