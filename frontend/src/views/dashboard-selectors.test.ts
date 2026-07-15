import { describe, it, expect } from "vitest";
import {
  getFocusedScenario,
  getCategoryRollups,
  getLivePoints,
  getDisplayedPoints,
  getModel,
  getCallCounts,
  isRunComplete,
} from "./dashboard-selectors";
import type { RunState, ScenarioState } from "../types";

function makeScenario(overrides: Partial<ScenarioState> = {}): ScenarioState {
  return {
    id: "SB-01",
    name: "Test",
    category: "surgical-edit",
    maxPoints: 10,
    status: "pending",
    toolCallCount: 0,
    bashCallCount: 0,
    editCallCount: 0,
    logs: [],
    streamBuffer: "",
    ...overrides,
  };
}

function makeState(overrides: Partial<RunState> = {}): RunState {
  return {
    runId: null,
    status: "idle",
    scenarios: [],
    activeScenarioId: null,
    focusedScenarioId: null,
    totalPoints: 0,
    maxPoints: 0,
    ...overrides,
  };
}

describe("getFocusedScenario", () => {
  it("returns focused scenario when focusedScenarioId is set", () => {
    const state = makeState({
      scenarios: [makeScenario({ id: "SB-01" }), makeScenario({ id: "SB-02" })],
      focusedScenarioId: "SB-02",
    });
    const result = getFocusedScenario(state);
    expect(result?.id).toBe("SB-02");
  });

  it("falls back to activeScenarioId when focusedScenarioId is null", () => {
    const state = makeState({
      scenarios: [makeScenario({ id: "SB-01" }), makeScenario({ id: "SB-02" })],
      activeScenarioId: "SB-01",
      focusedScenarioId: null,
    });
    const result = getFocusedScenario(state);
    expect(result?.id).toBe("SB-01");
  });

  it("returns undefined when no scenario matches", () => {
    const state = makeState({
      scenarios: [makeScenario({ id: "SB-01" })],
      focusedScenarioId: "SB-99",
    });
    const result = getFocusedScenario(state);
    expect(result).toBeUndefined();
  });
});

describe("getCategoryRollups", () => {
  it("aggregates points by category for completed scenarios", () => {
    const state = makeState({
      scenarios: [
        makeScenario({ category: "surgical-edit", status: "pass", points: 10, maxPoints: 10 }),
        makeScenario({ category: "surgical-edit", status: "pass", points: 8, maxPoints: 10 }),
        makeScenario({ category: "implementation", status: "fail", points: 0, maxPoints: 10 }),
      ],
    });
    const result = getCategoryRollups(state);
    expect(result).toHaveLength(2);
    expect(result.find((r) => r.category === "surgical-edit")).toEqual({
      category: "surgical-edit",
      points: 18,
      maxPoints: 20,
    });
    expect(result.find((r) => r.category === "implementation")).toEqual({
      category: "implementation",
      points: 0,
      maxPoints: 10,
    });
  });

  it("excludes pending and running scenarios", () => {
    const state = makeState({
      scenarios: [
        makeScenario({ category: "surgical-edit", status: "pending", points: 5, maxPoints: 10 }),
        makeScenario({ category: "surgical-edit", status: "running", points: 5, maxPoints: 10 }),
        makeScenario({ category: "surgical-edit", status: "pass", points: 10, maxPoints: 10 }),
      ],
    });
    const result = getCategoryRollups(state);
    expect(result).toHaveLength(1);
    expect(result[0].points).toBe(10);
  });
});

describe("getLivePoints", () => {
  it("sums all scenario points", () => {
    const state = makeState({
      scenarios: [
        makeScenario({ points: 10, maxPoints: 10 }),
        makeScenario({ points: 5, maxPoints: 10 }),
        makeScenario({ points: undefined, maxPoints: 10 }),
      ],
    });
    const result = getLivePoints(state);
    expect(result.total).toBe(15);
    expect(result.max).toBe(30);
  });

  it("handles empty scenarios", () => {
    const state = makeState({ scenarios: [] });
    const result = getLivePoints(state);
    expect(result.total).toBe(0);
    expect(result.max).toBe(0);
  });
});

describe("getDisplayedPoints", () => {
  it("returns live points when status is running", () => {
    const state = makeState({
      status: "running",
      scenarios: [makeScenario({ points: 10, maxPoints: 10 })],
      totalPoints: 0,
      maxPoints: 0,
    });
    const result = getDisplayedPoints(state);
    expect(result.total).toBe(10);
    expect(result.max).toBe(10);
  });

  it("returns live points when status is warming_up", () => {
    const state = makeState({
      status: "warming_up",
      scenarios: [makeScenario({ points: 5, maxPoints: 10 })],
      totalPoints: 0,
      maxPoints: 0,
    });
    const result = getDisplayedPoints(state);
    expect(result.total).toBe(5);
  });

  it("returns total points when status is done", () => {
    const state = makeState({
      status: "done",
      scenarios: [makeScenario({ points: 10, maxPoints: 10 })],
      totalPoints: 50,
      maxPoints: 100,
    });
    const result = getDisplayedPoints(state);
    expect(result.total).toBe(50);
    expect(result.max).toBe(100);
  });
});

describe("getModel", () => {
  it("returns model from state when available", () => {
    const state = makeState({ model: "gpt-4" });
    const result = getModel(state, undefined);
    expect(result).toBe("gpt-4");
  });

  it("falls back to focused scenario metrics", () => {
    const state = makeState({ model: null });
    const focused = makeScenario({
      liveMetrics: {
        model: "claude-3",
        requestCount: 1,
        promptTokens: 100,
        completionTokens: 50,
        totalTokens: 150,
        totalRequestTimeMs: 1000,
      },
    });
    const result = getModel(state, focused);
    expect(result).toBe("claude-3");
  });

  it("falls back to global metrics", () => {
    const state = makeState({
      model: null,
      globalMetrics: {
        model: "gpt-3.5",
        requestCount: 1,
        promptTokens: 100,
        completionTokens: 50,
        totalTokens: 150,
        totalRequestTimeMs: 1000,
      },
    });
    const result = getModel(state, undefined);
    expect(result).toBe("gpt-3.5");
  });

  it("returns null when no model found", () => {
    const state = makeState({ model: null });
    const result = getModel(state, undefined);
    expect(result).toBeNull();
  });
});

describe("getCallCounts", () => {
  it("returns counts from focused scenario", () => {
    const focused = makeScenario({
      toolCallCount: 10,
      bashCallCount: 5,
      editCallCount: 3,
    });
    const result = getCallCounts(focused);
    expect(result).toEqual({ tool: 10, bash: 5, edit: 3 });
  });

  it("returns zeros when focused is undefined", () => {
    const result = getCallCounts(undefined);
    expect(result).toEqual({ tool: 0, bash: 0, edit: 0 });
  });

  it("handles missing counts", () => {
    const focused = makeScenario({ toolCallCount: undefined });
    const result = getCallCounts(focused);
    expect(result.tool).toBe(0);
  });
});

describe("isRunComplete", () => {
  it("returns true for terminal statuses", () => {
    expect(isRunComplete("done")).toBe(true);
    expect(isRunComplete("stopped")).toBe(true);
    expect(isRunComplete("failed")).toBe(true);
  });

  it("returns false for non-terminal statuses", () => {
    expect(isRunComplete("idle")).toBe(false);
    expect(isRunComplete("warming_up")).toBe(false);
    expect(isRunComplete("running")).toBe(false);
  });
});
