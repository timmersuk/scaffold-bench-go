import { useEffect, useRef } from "react";
import type { BackendEvent } from "../types";

export function useOneshotSSE(
  runId: string | null,
  onEvent: (event: BackendEvent) => void
) {
  const esRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!runId) return;

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
    };

    return () => {
      es.close();
      esRef.current = null;
    };
  }, [runId, onEvent]);
}
