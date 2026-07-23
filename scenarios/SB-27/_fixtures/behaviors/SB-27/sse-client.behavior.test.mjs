import { test, expect } from "bun:test";
import { parseSSE } from "./sse-client.mjs";

test("parses first event from stream", () => {
  const events = parseSSE(["data: hello\n\ndata: world\n"]);
  expect(events).toContain("hello");
});

test("parses final event without trailing newline", () => {
  const events = parseSSE(["data: hello\n\ndata: world"]);
  expect(events).toContain("world");
});

test("parses multiple events correctly", () => {
  const events = parseSSE(["data: one\n\ndata: two\n\ndata: three"]);
  expect(events).toEqual(["one", "two", "three"]);
});

test("handles empty input", () => {
  const events = parseSSE([]);
  expect(events).toEqual([]);
});
