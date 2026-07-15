import { describe, it, expect } from "vitest";
import {
  oneshotStateReducer,
  initialState,
  hydrateFromLatestRun,
  type OneshotState,
  type OneshotAction,
} from "./oneshot-state-reducer";
import type { BackendEvent, OneshotResult } from "../types";

function makeState(overrides: Partial<OneshotState> = {}): OneshotState {
  return { ...initialState, ...overrides };
}

function makeEvent(type: string, seq: number, payload: any = {}): BackendEvent {
  return { type, seq, ts: Date.now(), payload } as BackendEvent;
}

describe("oneshot-state-reducer", () => {
  describe("initial state", () => {
    it("has correct initial values", () => {
      expect(initialState.runId).toBeNull();
      expect(initialState.status).toBe("idle");
      expect(initialState.promptIds).toEqual([]);
      expect(initialState.prompts).toEqual({});
      expect(initialState.lastSeenSeq).toBe(-1);
    });
  });

  describe("hydrate action", () => {
    it("replaces entire state", () => {
      const newState = makeState({
        runId: "run-1",
        status: "running",
        model: "gpt-4",
        promptIds: ["p1", "p2"],
        lastSeenSeq: 10,
      });
      const result = oneshotStateReducer(initialState, { type: "hydrate", state: newState });
      expect(result).toEqual(newState);
    });
  });

  describe("start action", () => {
    it("initializes run with prompts", () => {
      const result = oneshotStateReducer(initialState, {
        type: "start",
        runId: "run-1",
        model: "gpt-4",
        promptIds: ["p1", "p2"],
      });
      expect(result.runId).toBe("run-1");
      expect(result.status).toBe("running");
      expect(result.model).toBe("gpt-4");
      expect(result.promptIds).toEqual(["p1", "p2"]);
      expect(result.prompts["p1"].status).toBe("pending");
      expect(result.prompts["p2"].status).toBe("pending");
      expect(result.lastSeenSeq).toBe(-1);
    });
  });

  describe("event action", () => {
    describe("sequence tracking", () => {
      it("ignores events with seq <= lastSeenSeq", () => {
        const state = makeState({ lastSeenSeq: 10 });
        const event = makeEvent("oneshot_warmup_started", 5);
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.status).toBe("idle");
        expect(result.lastSeenSeq).toBe(10);
      });

      it("processes events with seq > lastSeenSeq", () => {
        const state = makeState({ lastSeenSeq: 10 });
        const event = makeEvent("oneshot_warmup_started", 11);
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.status).toBe("warming_up");
        expect(result.lastSeenSeq).toBe(11);
      });
    });

    describe("oneshot_run_started", () => {
      it("initializes run from event payload", () => {
        const event = makeEvent("oneshot_run_started", 1, {
          runId: "run-1",
          model: "gpt-4",
          promptIds: ["p1", "p2"],
        });
        const result = oneshotStateReducer(initialState, { type: "event", event });
        expect(result.runId).toBe("run-1");
        expect(result.status).toBe("warming_up");
        expect(result.model).toBe("gpt-4");
        expect(result.promptIds).toEqual(["p1", "p2"]);
      });
    });

    describe("warmup events", () => {
      it("transitions to warming_up on oneshot_warmup_started", () => {
        const event = makeEvent("oneshot_warmup_started", 1);
        const result = oneshotStateReducer(initialState, { type: "event", event });
        expect(result.status).toBe("warming_up");
      });

      it("transitions to running on oneshot_warmup_finished", () => {
        const state = makeState({ status: "warming_up" });
        const event = makeEvent("oneshot_warmup_finished", 2);
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.status).toBe("running");
      });
    });

    describe("oneshot_test_started", () => {
      it("sets prompt status to running", () => {
        const state = makeState({
          prompts: { p1: { id: "p1", status: "pending", output: "" } },
        });
        const event = makeEvent("oneshot_test_started", 1, { promptId: "p1" });
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.prompts["p1"].status).toBe("running");
        expect(result.prompts["p1"].output).toBe("");
      });
    });

    describe("oneshot_delta", () => {
      it("appends content to prompt output", () => {
        const state = makeState({
          prompts: { p1: { id: "p1", status: "running", output: "Hello" } },
        });
        const event = makeEvent("oneshot_delta", 1, { promptId: "p1", content: " world" });
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.prompts["p1"].output).toBe("Hello world");
      });

      it("ignores delta for unknown prompt", () => {
        const state = makeState({ prompts: {} });
        const event = makeEvent("oneshot_delta", 1, { promptId: "unknown", content: "test" });
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.prompts["unknown"]).toBeUndefined();
        expect(result.lastSeenSeq).toBe(1);
      });
    });

    describe("oneshot_test_finished", () => {
      it("sets prompt status to done on success", () => {
        const state = makeState({
          prompts: { p1: { id: "p1", status: "running", output: "" } },
        });
        const event = makeEvent("oneshot_test_finished", 1, {
          promptId: "p1",
          output: "<html>test</html>",
          wallTimeMs: 1000,
          firstTokenMs: 200,
          metrics: { promptTokens: 50, completionTokens: 100 },
          artifact: true,
          finishReason: "stop",
        });
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.prompts["p1"].status).toBe("done");
        expect(result.prompts["p1"].output).toBe("<html>test</html>");
        expect(result.prompts["p1"].wallTimeMs).toBe(1000);
        expect(result.prompts["p1"].firstTokenMs).toBe(200);
        expect(result.prompts["p1"].promptTokens).toBe(50);
        expect(result.prompts["p1"].completionTokens).toBe(100);
        expect(result.prompts["p1"].artifact).toBe(true);
        expect(result.prompts["p1"].artifactVersion).toBe(1);
        expect(result.prompts["p1"].finishReason).toBe("stop");
      });

      it("sets prompt status to failed on error", () => {
        const state = makeState({
          prompts: { p1: { id: "p1", status: "running", output: "" } },
        });
        const event = makeEvent("oneshot_test_finished", 1, {
          promptId: "p1",
          error: "Model timeout",
        });
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.prompts["p1"].status).toBe("failed");
        expect(result.prompts["p1"].error).toBe("Model timeout");
      });

      it("increments artifactVersion on successive artifacts", () => {
        const state = makeState({
          prompts: { p1: { id: "p1", status: "running", output: "", artifactVersion: 2 } },
        });
        const event = makeEvent("oneshot_test_finished", 1, {
          promptId: "p1",
          output: "<html>test</html>",
          artifact: true,
        });
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.prompts["p1"].artifactVersion).toBe(3);
      });

      it("preserves artifactVersion when no artifact", () => {
        const state = makeState({
          prompts: { p1: { id: "p1", status: "running", output: "", artifactVersion: 2 } },
        });
        const event = makeEvent("oneshot_test_finished", 1, {
          promptId: "p1",
          output: "plain text",
          artifact: false,
        });
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.prompts["p1"].artifactVersion).toBe(2);
      });
    });

    describe("run completion events", () => {
      it("sets status to done on oneshot_run_finished", () => {
        const state = makeState({ status: "running" });
        const event = makeEvent("oneshot_run_finished", 1);
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.status).toBe("done");
      });

      it("sets status to stopped on oneshot_run_stopped", () => {
        const state = makeState({ status: "running" });
        const event = makeEvent("oneshot_run_stopped", 1);
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.status).toBe("stopped");
      });

      it("sets status to failed on oneshot_run_failed", () => {
        const state = makeState({ status: "running" });
        const event = makeEvent("oneshot_run_failed", 1);
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.status).toBe("failed");
      });
    });

    describe("unknown events", () => {
      it("updates lastSeenSeq for unknown event types", () => {
        const state = makeState({ lastSeenSeq: 5 });
        const event = makeEvent("unknown_event", 10);
        const result = oneshotStateReducer(state, { type: "event", event });
        expect(result.lastSeenSeq).toBe(10);
      });
    });
  });

  describe("stop action", () => {
    it("sets status to stopped", () => {
      const state = makeState({ status: "running" });
      const result = oneshotStateReducer(state, { type: "stop" });
      expect(result.status).toBe("stopped");
    });
  });

  describe("reset action", () => {
    it("returns to initial state", () => {
      const state = makeState({
        runId: "run-1",
        status: "running",
        model: "gpt-4",
        lastSeenSeq: 10,
      });
      const result = oneshotStateReducer(state, { type: "reset" });
      expect(result).toEqual(initialState);
    });
  });
});

describe("hydrateFromLatestRun", () => {
  it("creates state from latest run results", () => {
    const results: OneshotResult[] = [
      {
        promptId: "p1",
        status: "done",
        output: "<html>test</html>",
        model: "gpt-4",
        finishReason: "stop",
        wallTimeMs: 1000,
        firstTokenMs: 200,
        promptTokens: 50,
        completionTokens: 100,
        hasArtifact: true,
      },
      {
        promptId: "p2",
        status: "failed",
        output: "",
        error: "Timeout",
        hasArtifact: false,
      },
    ];
    const result = hydrateFromLatestRun("run-1", "done", "gpt-4", ["p1", "p2"], results);
    expect(result.runId).toBe("run-1");
    expect(result.status).toBe("done");
    expect(result.model).toBe("gpt-4");
    expect(result.promptIds).toEqual(["p1", "p2"]);
    expect(result.prompts["p1"].status).toBe("done");
    expect(result.prompts["p1"].artifact).toBe(true);
    expect(result.prompts["p1"].artifactVersion).toBe(1);
    expect(result.prompts["p2"].status).toBe("failed");
    expect(result.prompts["p2"].error).toBe("Timeout");
    expect(result.prompts["p2"].artifact).toBe(false);
    expect(result.prompts["p2"].artifactVersion).toBe(0);
  });

  it("maps status strings correctly", () => {
    expect(hydrateFromLatestRun("r", "warming_up", null, [], []).status).toBe("warming_up");
    expect(hydrateFromLatestRun("r", "running", null, [], []).status).toBe("running");
    expect(hydrateFromLatestRun("r", "done", null, [], []).status).toBe("done");
    expect(hydrateFromLatestRun("r", "failed", null, [], []).status).toBe("failed");
    expect(hydrateFromLatestRun("r", "stopped", null, [], []).status).toBe("stopped");
    expect(hydrateFromLatestRun("r", "unknown", null, [], []).status).toBe("idle");
  });

  it("handles null model", () => {
    const result = hydrateFromLatestRun("run-1", "done", null, [], []);
    expect(result.model).toBeNull();
  });

  it("handles undefined model", () => {
    const result = hydrateFromLatestRun("run-1", "done", undefined, [], []);
    expect(result.model).toBeNull();
  });
});
