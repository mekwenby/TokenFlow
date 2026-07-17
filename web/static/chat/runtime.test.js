import assert from "node:assert/strict";
import test from "node:test";

import { formatCompactNumber } from "../core/format.js";
import { AUTO_TITLE_DELAY_MS, canRetryStream, chatViewportFrame, createRequestID, isNearBottom, messageEvents, normalizeMaxToolCalls, normalizeTokenUsage, parseSSE, readSSE, scheduleAutoTitle, tokenUsageDetailKeys } from "./runtime.js";

test("automatic titles are scheduled two seconds after reply completion", () => {
  let receivedDelay = 0;
  let called = false;
  const timer = scheduleAutoTitle(() => { called = true; }, (callback, delay) => {
    receivedDelay = delay;
    callback();
    return 42;
  });
  assert.equal(AUTO_TITLE_DELAY_MS, 2000);
  assert.equal(receivedDelay, 2000);
  assert.equal(called, true);
  assert.equal(timer, 42);
});

test("parseSSE reads named events and multi-line JSON data", () => {
  const event = parseSSE("event: delta\ndata: {\"content\":\ndata: \"hello\"}");
  assert.deepEqual(event, { event: "delta", data: { content: "hello" } });
});

test("parseSSE rejects invalid JSON without evaluating it", () => {
  assert.throws(() => parseSSE("event: delta\ndata: not-json"), /Invalid streaming response/);
});

test("readSSE emits a final event without a trailing blank line", async () => {
  const encoder = new TextEncoder();
  const response = new Response(new ReadableStream({
    start(controller) {
      controller.enqueue(encoder.encode("event: delta\ndata: {\"content\":\"tail\"}"));
      controller.close();
    },
  }));
  const events = [];
  await readSSE(response, (event, data) => events.push({ event, data }));
  assert.deepEqual(events, [{ event: "delta", data: { content: "tail" } }]);
});

test("readSSE reports a missing response stream", async () => {
  await assert.rejects(() => readSSE({}, () => {}), /Streaming response is unavailable/);
});

test("messageEvents keeps events scoped to their assistant message", () => {
  const first = { metadata: JSON.stringify({ events: [{ type: "thinking", content: "one" }] }) };
  const second = { metadata: JSON.stringify({ events: [{ type: "tool_result", result: "two" }] }) };
  assert.equal(messageEvents(first)[0].content, "one");
  assert.equal(messageEvents(second)[0].result, "two");
  assert.deepEqual(messageEvents({ metadata: "invalid" }), []);
});

test("scroll following only applies near the bottom", () => {
  assert.equal(isNearBottom({ scrollHeight: 1000, scrollTop: 620, clientHeight: 300 }), true);
  assert.equal(isNearBottom({ scrollHeight: 1000, scrollTop: 400, clientHeight: 300 }), false);
});

test("chat viewport follows the iOS keyboard and clears stale offsets after it closes", () => {
  assert.deepEqual(chatViewportFrame({
    viewportHeight: 420,
    viewportOffsetTop: 96,
    baselineHeight: 780,
    editableFocused: true,
  }), { height: 420, offsetTop: 96, keyboardOpen: true });
  assert.deepEqual(chatViewportFrame({
    viewportHeight: 780,
    viewportOffsetTop: 47,
    baselineHeight: 780,
    editableFocused: true,
  }), { height: 780, offsetTop: 0, keyboardOpen: false });
});

test("tool call limits are normalized to the server range", () => {
  assert.equal(normalizeMaxToolCalls("bad"), 6);
  assert.equal(normalizeMaxToolCalls(-4), 0);
  assert.equal(normalizeMaxToolCalls(99), 20);
});

test("token usage total includes input and output without double-counting cache", () => {
  assert.deepEqual(normalizeTokenUsage({
    input_tokens: 10000,
    output_tokens: 2400,
    cache_read_tokens: 8000,
    cache_creation_tokens: 500,
  }), {
    input_tokens: 10000,
    output_tokens: 2400,
    cache_read_tokens: 8000,
    cache_creation_tokens: 500,
    total_tokens: 12400,
  });
});

test("token usage normalizes invalid and negative counts", () => {
  assert.equal(normalizeTokenUsage(null), null);
  assert.deepEqual(normalizeTokenUsage({
    input_tokens: -1,
    output_tokens: "12.9",
    cache_read_tokens: Number.NaN,
    cache_creation_tokens: Number.POSITIVE_INFINITY,
  }), {
    input_tokens: 0,
    output_tokens: 12,
    cache_read_tokens: 0,
    cache_creation_tokens: 0,
    total_tokens: 12,
  });
});

test("token usage details always show cache hits and omit zero cache writes", () => {
  assert.deepEqual(tokenUsageDetailKeys({ input_tokens: 10000, output_tokens: 2400 }), [
    "total_tokens",
    "input_tokens",
    "output_tokens",
    "cache_read_tokens",
  ]);
  assert.deepEqual(tokenUsageDetailKeys({ input_tokens: 10, output_tokens: 2, cache_read_tokens: 8, cache_creation_tokens: 3 }), [
    "total_tokens",
    "input_tokens",
    "output_tokens",
    "cache_read_tokens",
    "cache_creation_tokens",
  ]);
});

test("token usage summary uses compact K M B formatting", () => {
  assert.equal(formatCompactNumber(999), "999");
  assert.equal(formatCompactNumber(12400), "12.4K");
  assert.equal(formatCompactNumber(2400000), "2.4M");
});

test("request IDs use the browser UUID implementation when available", () => {
  assert.equal(createRequestID({ randomUUID: () => "request-123" }), "request-123");
});

test("transport retries stop after output, server errors, or user cancellation", () => {
  const networkError = new TypeError("Failed to fetch");
  assert.equal(canRetryStream({ attempt: 0, error: networkError }), true);
  assert.equal(canRetryStream({ attempt: 2, error: networkError }), false);
  assert.equal(canRetryStream({ attempt: 0, hasDelta: true, error: networkError }), false);
  assert.equal(canRetryStream({ attempt: 0, serverError: true, error: networkError }), false);
  assert.equal(canRetryStream({ attempt: 0, stopRequested: true, error: networkError }), false);
  assert.equal(canRetryStream({ attempt: 0, error: Object.assign(new Error("busy"), { retryable: true }) }), true);
});
