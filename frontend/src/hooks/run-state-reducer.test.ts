import { describe, it, expect } from "vitest";
import { reducer, INITIAL_REDUCER_STATE, type ReducerState } from "./run-state-reducer";
import type { PersistedEvent } from "../types";

function makeState(overrides: Partial<ReducerState> = {}): ReducerState {
  return { ...INITIAL_REDUCER_STATE, ...overrides };
}

function makeEvent(type: string, overrides: any = {}): PersistedEvent {
  return { type, ts: Date.now(), ...overrides } as PersistedEvent;
}

describe("run-state-reducer", () => {
  describe("initial state", () => {
    it("has correct initial values", () => {
      expect(INITIAL_REDUCER_STATE.runId).toBeNull();
      expect(INITIAL_REDUCER_STATE.status).toBe("idle");
      expect(INITIAL_REDUCER_STATE.scenarios).toEqual([]);
      expect(INITIAL_REDUCER_STATE._nextLogId).toBe(1);
    });
  });

  describe("_reset action", () => {
    it("resets state but preserves log ID counter", () => {
      const state = makeState({ runId: "run-1", status: "running", _nextLogId: 50 });
      const result = reducer(state, { type: "_reset" });
      expect(result.runId).toBeNull();
      expect(result.status).toBe("idle");
      expect(result._nextLogId).toBe(50);
    });
  });

  describe("_focus action", () => {
    it("updates focusedScenarioId", () => {
      const state = makeState({ activeScenarioId: "SB-01", focusedScenarioId: "SB-01" });
      const result = reducer(state, { type: "_focus", id: "SB-02" });
      expect(result.focusedScenarioId).toBe("SB-02");
      expect(result.activeScenarioId).toBe("SB-01");
    });
  });

  describe("run_started", () => {
    it("initializes scenarios from event", () => {
      const event = makeEvent("run_started", {
        runId: "run-1",
        scenarioIds: ["SB-01", "SB-02"],
        model: "gpt-4",
      });
      const result = reducer(INITIAL_REDUCER_STATE, event);
      expect(result.runId).toBe("run-1");
      expect(result.status).toBe("warming_up");
      expect(result.scenarios).toHaveLength(2);
      expect(result.scenarios[0].id).toBe("SB-01");
      expect(result.scenarios[0].status).toBe("pending");
      expect(result.model).toBe("gpt-4");
    });

    it("preserves log ID counter", () => {
      const state = makeState({ _nextLogId: 100 });
      const event = makeEvent("run_started", { runId: "run-1", scenarioIds: ["SB-01"] });
      const result = reducer(state, event);
      expect(result._nextLogId).toBe(100);
    });
  });

  describe("model_warmup", () => {
    it("transitions to warming_up on model_warmup_started", () => {
      const state = makeState({ status: "idle" });
      const result = reducer(state, makeEvent("model_warmup_started"));
      expect(result.status).toBe("warming_up");
    });

    it("transitions to running on model_warmup_finished", () => {
      const state = makeState({ status: "warming_up" });
      const result = reducer(state, makeEvent("model_warmup_finished"));
      expect(result.status).toBe("running");
    });
  });

  describe("scenario_started", () => {
    it("updates scenario metadata and status", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "pending", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "" }],
      });
      const event = makeEvent("scenario_started", {
        scenarioId: "SB-01",
        name: "Fix the bug",
        category: "surgical-edit",
        maxPoints: 10,
      });
      const result = reducer(state, event);
      expect(result.scenarios[0].name).toBe("Fix the bug");
      expect(result.scenarios[0].category).toBe("surgical-edit");
      expect(result.scenarios[0].maxPoints).toBe(10);
      expect(result.scenarios[0].status).toBe("running");
      expect(result.activeScenarioId).toBe("SB-01");
    });

    it("auto-focuses on first scenario", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "pending", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "" }],
        activeScenarioId: null,
        focusedScenarioId: null,
      });
      const event = makeEvent("scenario_started", { scenarioId: "SB-01" });
      const result = reducer(state, event);
      expect(result.focusedScenarioId).toBe("SB-01");
    });

    it("preserves user focus when user has manually focused elsewhere", () => {
      const state = makeState({
        scenarios: [
          { id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "" },
          { id: "SB-02", name: "", category: "", maxPoints: 0, status: "pending", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "" },
        ],
        activeScenarioId: "SB-01",
        focusedScenarioId: "SB-02",
      });
      const event = makeEvent("scenario_started", { scenarioId: "SB-02" });
      const result = reducer(state, event);
      expect(result.focusedScenarioId).toBe("SB-02");
    });
  });

  describe("assistant_delta", () => {
    it("appends content to stream buffer", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "Hello", reasoningBuffer: "" }],
      });
      const event = makeEvent("assistant_delta", { scenarioId: "SB-01", content: " world" });
      const result = reducer(state, event);
      expect(result.scenarios[0].streamBuffer).toBe("Hello world");
    });

    it("calculates firstTokenMs on first non-empty delta", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", startedAt: 1000, toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "", firstTokenMs: undefined }],
      });
      const event = makeEvent("assistant_delta", { scenarioId: "SB-01", content: "Hello", ts: 1500 });
      const result = reducer(state, event);
      expect(result.scenarios[0].firstTokenMs).toBe(500);
    });

    it("does not update firstTokenMs if already set", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", startedAt: 1000, toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "", firstTokenMs: 200 }],
      });
      const event = makeEvent("assistant_delta", { scenarioId: "SB-01", content: "world", ts: 1500 });
      const result = reducer(state, event);
      expect(result.scenarios[0].firstTokenMs).toBe(200);
    });

    it("does not calculate firstTokenMs for empty content", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", startedAt: 1000, toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "", firstTokenMs: undefined }],
      });
      const event = makeEvent("assistant_delta", { scenarioId: "SB-01", content: "   ", ts: 1500 });
      const result = reducer(state, event);
      expect(result.scenarios[0].firstTokenMs).toBeUndefined();
    });
  });

  describe("assistant", () => {
    it("flushes stream buffer to logs", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "Hello world", reasoningBuffer: "" }],
        _nextLogId: 1,
      });
      const event = makeEvent("assistant", { scenarioId: "SB-01", content: "Hello world" });
      const result = reducer(state, event);
      expect(result.scenarios[0].streamBuffer).toBe("");
      expect(result.scenarios[0].logs).toHaveLength(1);
      expect(result.scenarios[0].logs[0].kind).toBe("assistant");
      expect(result.scenarios[0].logs[0].text).toBe("Hello world");
    });

    it("increments log ID when adding entry", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "Test", reasoningBuffer: "" }],
        _nextLogId: 10,
      });
      const event = makeEvent("assistant", { scenarioId: "SB-01", content: "Test" });
      const result = reducer(state, event);
      expect(result._nextLogId).toBe(11);
    });

    it("does not add log entry for empty content", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "" }],
        _nextLogId: 1,
      });
      const event = makeEvent("assistant", { scenarioId: "SB-01", content: "" });
      const result = reducer(state, event);
      expect(result.scenarios[0].logs).toHaveLength(0);
      expect(result._nextLogId).toBe(1);
    });
  });

  describe("tool_call", () => {
    it("creates tool log entry with command label", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "" }],
        _nextLogId: 1,
      });
      const event = makeEvent("tool_call", {
        scenarioId: "SB-01",
        call: { name: "bash", args: JSON.stringify({ command: "ls -la" }) },
      });
      const result = reducer(state, event);
      expect(result.scenarios[0].logs).toHaveLength(1);
      expect(result.scenarios[0].logs[0].label).toBe("cmd");
      expect(result.scenarios[0].logs[0].text).toBe("$ ls -la");
    });

    it("increments tool and bash counters", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "" }],
      });
      const event = makeEvent("tool_call", {
        scenarioId: "SB-01",
        call: { name: "bash", args: JSON.stringify({ command: "echo test" }) },
      });
      const result = reducer(state, event);
      expect(result.scenarios[0].toolCallCount).toBe(1);
      expect(result.scenarios[0].bashCallCount).toBe(1);
    });

    it("increments edit counter for edit tools", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "" }],
      });
      const event = makeEvent("tool_call", {
        scenarioId: "SB-01",
        call: { name: "edit", args: JSON.stringify({ path: "file.ts" }) },
      });
      const result = reducer(state, event);
      expect(result.scenarios[0].editCallCount).toBe(1);
      expect(result.scenarios[0].bashCallCount).toBe(0);
    });

    it("flushes stream buffer before tool entry", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "Thinking...", reasoningBuffer: "" }],
        _nextLogId: 1,
      });
      const event = makeEvent("tool_call", {
        scenarioId: "SB-01",
        call: { name: "bash", args: JSON.stringify({ command: "ls" }) },
      });
      const result = reducer(state, event);
      expect(result.scenarios[0].logs).toHaveLength(2);
      expect(result.scenarios[0].logs[0].kind).toBe("assistant");
      expect(result.scenarios[0].logs[0].text).toBe("Thinking...");
      expect(result.scenarios[0].logs[1].kind).toBe("tool");
      expect(result.scenarios[0].streamBuffer).toBe("");
    });
  });

  describe("tool_result", () => {
    it("creates stdout entry for normal result", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "" }],
        _nextLogId: 1,
      });
      const event = makeEvent("tool_result", { scenarioId: "SB-01", result: "file1.txt\nfile2.txt" });
      const result = reducer(state, event);
      expect(result.scenarios[0].logs).toHaveLength(1);
      expect(result.scenarios[0].logs[0].kind).toBe("stdout");
      expect(result.scenarios[0].logs[0].text).toBe("file1.txt\nfile2.txt");
    });

    it("creates stderr entry for error result", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "" }],
      });
      const event = makeEvent("tool_result", { scenarioId: "SB-01", result: "Error: file not found" });
      const result = reducer(state, event);
      expect(result.scenarios[0].logs[0].kind).toBe("stderr");
    });

    it("truncates long results to 500 chars", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "" }],
      });
      const longResult = "x".repeat(1000);
      const event = makeEvent("tool_result", { scenarioId: "SB-01", result: longResult });
      const result = reducer(state, event);
      expect(result.scenarios[0].logs[0].text).toHaveLength(500);
    });
  });

  describe("scenario_finished", () => {
    it("updates scenario status and metrics", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 5, bashCallCount: 2, editCallCount: 1, logs: [], streamBuffer: "", reasoningBuffer: "" }],
      });
      const event = makeEvent("scenario_finished", {
        scenarioId: "SB-01",
        status: "pass",
        points: 10,
        wallTimeMs: 5000,
        toolCallCount: 5,
        evaluation: { checks: [] },
      });
      const result = reducer(state, event);
      expect(result.scenarios[0].status).toBe("pass");
      expect(result.scenarios[0].points).toBe(10);
      expect(result.scenarios[0].wallTimeMs).toBe(5000);
      expect(result.scenarios[0].evaluation).toBeDefined();
    });

    it("flushes remaining stream buffer", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "Final thoughts", reasoningBuffer: "" }],
        _nextLogId: 1,
      });
      const event = makeEvent("scenario_finished", { scenarioId: "SB-01", status: "pass", points: 10, wallTimeMs: 1000, toolCallCount: 0 });
      const result = reducer(state, event);
      expect(result.scenarios[0].streamBuffer).toBe("");
      expect(result.scenarios[0].logs).toHaveLength(1);
      expect(result.scenarios[0].logs[0].text).toBe("Final thoughts");
    });

    it("clears activeScenarioId when finishing active scenario", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "" }],
        activeScenarioId: "SB-01",
      });
      const event = makeEvent("scenario_finished", { scenarioId: "SB-01", status: "pass", points: 10, wallTimeMs: 1000, toolCallCount: 0 });
      const result = reducer(state, event);
      expect(result.activeScenarioId).toBeNull();
    });
  });

  describe("model_metrics", () => {
    it("updates global and scenario metrics", () => {
      const state = makeState({
        scenarios: [{ id: "SB-01", name: "", category: "", maxPoints: 0, status: "running", toolCallCount: 0, bashCallCount: 0, editCallCount: 0, logs: [], streamBuffer: "", reasoningBuffer: "" }],
      });
      const event = makeEvent("model_metrics", {
        scenarioId: "SB-01",
        metrics: { promptTokens: 100, completionTokens: 50, totalTokens: 150 },
      });
      const result = reducer(state, event);
      expect(result.globalMetrics).toEqual({ promptTokens: 100, completionTokens: 50, totalTokens: 150 });
      expect(result.scenarios[0].liveMetrics).toEqual({ promptTokens: 100, completionTokens: 50, totalTokens: 150 });
    });
  });

  describe("run completion", () => {
    it("sets status to done on run_finished", () => {
      const state = makeState({ status: "running" });
      const event = makeEvent("run_finished", { totalPoints: 50, maxPoints: 100 });
      const result = reducer(state, event);
      expect(result.status).toBe("done");
      expect(result.totalPoints).toBe(50);
      expect(result.maxPoints).toBe(100);
    });

    it("sets status to stopped on run_stopped", () => {
      const state = makeState({ status: "running" });
      const result = reducer(state, makeEvent("run_stopped"));
      expect(result.status).toBe("stopped");
    });

    it("sets status to failed on run_failed", () => {
      const state = makeState({ status: "running" });
      const result = reducer(state, makeEvent("run_failed"));
      expect(result.status).toBe("failed");
    });
  });
});
