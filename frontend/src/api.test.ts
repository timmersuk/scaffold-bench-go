import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { api, ApiError } from "./api";

const mockFetch = vi.fn();
global.fetch = mockFetch;

beforeEach(() => {
  mockFetch.mockReset();
});

afterEach(() => {
  vi.restoreAllMocks();
});

function mockResponse(data: unknown, ok = true, status = 200) {
  return Promise.resolve({
    ok,
    status,
    json: () => Promise.resolve(data),
  });
}

describe("api client", () => {
  describe("GET requests", () => {
    it("getScenarios fetches /api/scenarios", async () => {
      mockFetch.mockReturnValue(mockResponse([{ id: "SB-01" }]));
      const result = await api.getScenarios();
      expect(mockFetch).toHaveBeenCalledWith("/api/scenarios", { signal: undefined });
      expect(result).toEqual([{ id: "SB-01" }]);
    });

    it("getModels fetches /api/models", async () => {
      mockFetch.mockReturnValue(mockResponse({ local: [], remote: [] }));
      const result = await api.getModels();
      expect(mockFetch).toHaveBeenCalledWith("/api/models", { signal: undefined });
      expect(result).toEqual({ local: [], remote: [] });
    });

    it("listRuns fetches /api/runs", async () => {
      mockFetch.mockReturnValue(mockResponse([{ id: "run-1" }]));
      const result = await api.listRuns();
      expect(mockFetch).toHaveBeenCalledWith("/api/runs", { signal: undefined });
      expect(result).toEqual([{ id: "run-1" }]);
    });

    it("getRun fetches /api/runs/:id", async () => {
      mockFetch.mockReturnValue(mockResponse({ id: "run-1" }));
      const result = await api.getRun("run-1");
      expect(mockFetch).toHaveBeenCalledWith("/api/runs/run-1", { signal: undefined });
      expect(result).toEqual({ id: "run-1" });
    });

    it("activeRun fetches /api/runs/active", async () => {
      mockFetch.mockReturnValue(mockResponse({ runId: "run-1" }));
      const result = await api.activeRun();
      expect(mockFetch).toHaveBeenCalledWith("/api/runs/active", { signal: undefined });
      expect(result).toEqual({ runId: "run-1" });
    });

    it("getRunEvents fetches /api/runs/:id/events with fromSeq", async () => {
      mockFetch.mockReturnValue(mockResponse([{ type: "run_started" }]));
      const result = await api.getRunEvents("run-1", 10);
      expect(mockFetch).toHaveBeenCalledWith("/api/runs/run-1/events?fromSeq=10", { signal: undefined });
      expect(result).toEqual([{ type: "run_started" }]);
    });

    it("getReportData fetches /api/report/data", async () => {
      mockFetch.mockReturnValue(mockResponse({ solveRate: 0.8 }));
      const result = await api.getReportData();
      expect(mockFetch).toHaveBeenCalledWith("/api/report/data", { signal: undefined });
      expect(result).toEqual({ solveRate: 0.8 });
    });

    it("oneshotTests fetches /api/oneshot/tests", async () => {
      mockFetch.mockReturnValue(mockResponse([{ id: "p1" }]));
      const result = await api.oneshotTests();
      expect(mockFetch).toHaveBeenCalledWith("/api/oneshot/tests", { signal: undefined });
      expect(result).toEqual([{ id: "p1" }]);
    });

    it("latestOneshot fetches /api/oneshot/runs/latest", async () => {
      mockFetch.mockReturnValue(mockResponse({ runId: "run-1" }));
      const result = await api.latestOneshot();
      expect(mockFetch).toHaveBeenCalledWith("/api/oneshot/runs/latest", { signal: undefined });
      expect(result).toEqual({ runId: "run-1" });
    });

    it("getConfig fetches /api/config", async () => {
      mockFetch.mockReturnValue(mockResponse({ localEndpoint: "http://localhost" }));
      const result = await api.getConfig();
      expect(mockFetch).toHaveBeenCalledWith("/api/config", { signal: undefined });
      expect(result).toEqual({ localEndpoint: "http://localhost" });
    });

    it("passes AbortSignal to GET requests", async () => {
      mockFetch.mockReturnValue(mockResponse([]));
      const controller = new AbortController();
      await api.getScenarios(controller.signal);
      expect(mockFetch).toHaveBeenCalledWith("/api/scenarios", { signal: controller.signal });
    });
  });

  describe("POST requests", () => {
    it("createRun posts to /api/runs", async () => {
      mockFetch.mockReturnValue(mockResponse({ runId: "run-1" }));
      const body = { scenarioIds: ["SB-01"], modelId: "gpt-4" };
      const result = await api.createRun(body);
      expect(mockFetch).toHaveBeenCalledWith("/api/runs", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      });
      expect(result).toEqual({ runId: "run-1" });
    });

    it("stopRun posts to /api/runs/:id/stop", async () => {
      mockFetch.mockReturnValue(mockResponse({ ok: true }));
      const result = await api.stopRun("run-1");
      expect(mockFetch).toHaveBeenCalledWith("/api/runs/run-1/stop", {
        method: "POST",
        headers: undefined,
        body: undefined,
      });
      expect(result).toEqual({ ok: true });
    });

    it("clearRuns posts to /api/runs/clear", async () => {
      mockFetch.mockReturnValue(mockResponse({ ok: true }));
      const result = await api.clearRuns();
      expect(mockFetch).toHaveBeenCalledWith("/api/runs/clear", {
        method: "POST",
        headers: undefined,
        body: undefined,
      });
      expect(result).toEqual({ ok: true });
    });

    it("startOneshot posts to /api/oneshot/runs", async () => {
      mockFetch.mockReturnValue(mockResponse({ runId: "run-1" }));
      const body = { modelId: "gpt-4", promptIds: ["p1", "p2"] };
      const result = await api.startOneshot(body);
      expect(mockFetch).toHaveBeenCalledWith("/api/oneshot/runs", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      });
      expect(result).toEqual({ runId: "run-1" });
    });

    it("stopOneshot posts to /api/oneshot/runs/:id/stop", async () => {
      mockFetch.mockReturnValue(mockResponse({ ok: true, runId: "run-1", status: "stopped" }));
      const result = await api.stopOneshot("run-1");
      expect(mockFetch).toHaveBeenCalledWith("/api/oneshot/runs/run-1/stop", {
        method: "POST",
        headers: undefined,
        body: undefined,
      });
      expect(result).toEqual({ ok: true, runId: "run-1", status: "stopped" });
    });
  });

  describe("PUT requests", () => {
    it("updateConfig puts to /api/config", async () => {
      mockFetch.mockReturnValue(mockResponse({ localEndpoint: "http://localhost:8080" }));
      const body = { localEndpoint: "http://localhost:8080" };
      const result = await api.updateConfig(body);
      expect(mockFetch).toHaveBeenCalledWith("/api/config", {
        method: "PUT",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      });
      expect(result).toEqual({ localEndpoint: "http://localhost:8080" });
    });
  });

  describe("URL generation", () => {
    it("oneshotArtifactUrl returns correct URL without version", () => {
      const url = api.oneshotArtifactUrl("p1");
      expect(url).toBe("/api/oneshot/artifacts/p1");
    });

    it("oneshotArtifactUrl returns correct URL with version", () => {
      const url = api.oneshotArtifactUrl("p1", 3);
      expect(url).toBe("/api/oneshot/artifacts/p1?v=3");
    });
  });

  describe("error handling", () => {
    it("throws ApiError on non-ok response", async () => {
      mockFetch.mockReturnValue(mockResponse({ error: "Not found" }, false, 404));
      await expect(api.getScenarios()).rejects.toThrow(ApiError);
      await expect(api.getScenarios()).rejects.toThrow("GET /scenarios -> 404");
    });

    it("includes status code in ApiError", async () => {
      mockFetch.mockReturnValue(mockResponse({}, false, 500));
      try {
        await api.getModels();
        expect.fail("Should have thrown");
      } catch (e) {
        expect(e).toBeInstanceOf(ApiError);
        expect((e as ApiError).status).toBe(500);
      }
    });
  });
});
