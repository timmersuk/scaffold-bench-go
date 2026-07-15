import { describe, it, expect } from "vitest";
import { formatElapsed, formatWallTime, formatNowHHMMSS } from "./format";

describe("formatElapsed", () => {
  it("formats milliseconds as MM:SS", () => {
    expect(formatElapsed(0)).toBe("00:00");
    expect(formatElapsed(1000)).toBe("00:01");
    expect(formatElapsed(59000)).toBe("00:59");
    expect(formatElapsed(60000)).toBe("01:00");
    expect(formatElapsed(61000)).toBe("01:01");
    expect(formatElapsed(3600000)).toBe("60:00");
  });

  it("pads minutes and seconds with zeros", () => {
    expect(formatElapsed(5000)).toBe("00:05");
    expect(formatElapsed(305000)).toBe("05:05");
  });

  it("floors milliseconds to seconds", () => {
    expect(formatElapsed(1500)).toBe("00:01");
    expect(formatElapsed(1999)).toBe("00:01");
  });
});

describe("formatWallTime", () => {
  it("formats seconds with s suffix for < 60s", () => {
    expect(formatWallTime(0)).toBe("0s");
    expect(formatWallTime(30)).toBe("30s");
    expect(formatWallTime(59)).toBe("59s");
  });

  it("formats minutes and seconds for 60s to 1h", () => {
    expect(formatWallTime(60)).toBe("1m 00s");
    expect(formatWallTime(90)).toBe("1m 30s");
    expect(formatWallTime(3599)).toBe("59m 59s");
  });

  it("formats hours and minutes for >= 1h", () => {
    expect(formatWallTime(3600)).toBe("1h 0m");
    expect(formatWallTime(3661)).toBe("1h 1m");
    expect(formatWallTime(7200)).toBe("2h 0m");
  });

  it("floors to whole seconds", () => {
    expect(formatWallTime(30.7)).toBe("30s");
    expect(formatWallTime(89.9)).toBe("1m 29s");
  });
});

describe("formatNowHHMMSS", () => {
  it("returns time in HH:MM:SS format", () => {
    const result = formatNowHHMMSS();
    expect(result).toMatch(/^\d{2}:\d{2}:\d{2}$/);
  });

  it("returns current time", () => {
    const now = new Date();
    const result = formatNowHHMMSS();
    const [h, m, s] = result.split(":").map(Number);
    
    expect(h).toBe(now.getUTCHours());
    expect(m).toBe(now.getUTCMinutes());
    expect(s).toBeGreaterThanOrEqual(now.getUTCSeconds() - 1);
    expect(s).toBeLessThanOrEqual(now.getUTCSeconds() + 1);
  });
});
