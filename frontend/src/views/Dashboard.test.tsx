import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom";
import { Dashboard } from "./Dashboard";
import { api } from "../api";

vi.mock("../api");
const mockApi = vi.mocked(api);

beforeEach(() => {
  vi.clearAllMocks();
  mockApi.activeRun.mockResolvedValue({ runId: null });
  mockApi.getRunEvents.mockResolvedValue([]);
  (globalThis as any).fetch = vi.fn().mockResolvedValue({ ok: true });
  (globalThis as any).EventSource = vi.fn().mockImplementation(() => ({
    close: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }));
});

describe("Dashboard", () => {
  it("renders without crashing", () => {
    render(<Dashboard onStartRun={vi.fn()} onHistory={vi.fn()} />);
    expect(screen.getByText("Dashboard")).toBeInTheDocument();
  });
});
