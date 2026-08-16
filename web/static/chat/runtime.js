export const AUTO_TITLE_DELAY_MS = 2000;
export const CLIENT_RETRY_DELAYS_MS = [500, 1500];

export function scheduleAutoTitle(callback, setTimer = setTimeout) {
  return setTimer(callback, AUTO_TITLE_DELAY_MS);
}

export function parseSSE(raw) {
  const lines = String(raw || "").split(/\r?\n/);
  let event = "message";
  const dataLines = [];
  for (const line of lines) {
    if (!line || line.startsWith(":")) continue;
    if (line.startsWith("event:")) event = line.slice(6).trim() || "message";
    if (line.startsWith("data:")) dataLines.push(line.slice(5).replace(/^ /, ""));
  }
  if (!dataLines.length) return null;
  const payload = dataLines.join("\n");
  try {
    return { event, data: JSON.parse(payload) };
  } catch {
    throw new Error("Invalid streaming response");
  }
}

export async function readSSE(response, onEvent) {
  if (!response?.body?.getReader) throw new Error("Streaming response is unavailable");
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  const emitBlocks = () => {
    const parts = buffer.split(/\r?\n\r?\n/);
    buffer = parts.pop() || "";
    for (const part of parts) {
      if (!part.trim()) continue;
      const parsed = parseSSE(part);
      if (parsed) onEvent(parsed.event, parsed.data);
    }
  };

  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    emitBlocks();
  }
  buffer += decoder.decode();
  if (buffer.trim()) {
    const tail = buffer;
    buffer = "";
    const parsed = parseSSE(tail);
    if (parsed) onEvent(parsed.event, parsed.data);
  }
}

export function messageEvents(message) {
  if (!message?.metadata) return [];
  try {
    const metadata = typeof message.metadata === "string" ? JSON.parse(message.metadata) : message.metadata;
    return Array.isArray(metadata?.events) ? metadata.events : [];
  } catch {
    return [];
  }
}

export function isNearBottom(node, threshold = 96) {
  if (!node) return true;
  return node.scrollHeight - node.scrollTop - node.clientHeight <= threshold;
}

export function chatViewportFrame({ viewportHeight, viewportOffsetTop = 0, baselineHeight, editableFocused = false } = {}) {
  const height = Math.max(1, Math.round(Number(viewportHeight) || 1));
  const baseline = Math.max(height, Math.round(Number(baselineHeight) || height));
  const keyboardOpen = editableFocused && baseline - height > 80;
  return {
    height,
    offsetTop: keyboardOpen ? Math.max(0, Math.round(Number(viewportOffsetTop) || 0)) : 0,
    keyboardOpen,
  };
}

export function normalizeMaxToolCalls(value) {
  const parsed = Number.parseInt(String(value ?? 7), 10);
  if (!Number.isFinite(parsed)) return 7;
  return Math.min(20, Math.max(0, parsed));
}

export function normalizeTokenUsage(usage) {
  if (!usage || typeof usage !== "object") return null;
  const tokenCount = (value) => {
    const parsed = Number(value || 0);
    return Number.isFinite(parsed) && parsed > 0 ? Math.trunc(parsed) : 0;
  };
  const normalized = {
    input_tokens: tokenCount(usage.input_tokens),
    output_tokens: tokenCount(usage.output_tokens),
    cache_read_tokens: tokenCount(usage.cache_read_tokens),
    cache_creation_tokens: tokenCount(usage.cache_creation_tokens),
  };
  return {
    ...normalized,
    total_tokens: normalized.input_tokens + normalized.output_tokens,
  };
}

export function tokenUsageDetailKeys(usage) {
  const normalized = normalizeTokenUsage(usage);
  if (!normalized) return [];
  const keys = ["total_tokens", "input_tokens", "output_tokens", "cache_read_tokens"];
  if (normalized.cache_creation_tokens > 0) keys.push("cache_creation_tokens");
  return keys;
}

export function createRequestID(cryptoObject = globalThis.crypto) {
  if (typeof cryptoObject?.randomUUID === "function") return cryptoObject.randomUUID();
  const bytes = new Uint8Array(16);
  if (typeof cryptoObject?.getRandomValues === "function") cryptoObject.getRandomValues(bytes);
  else for (let index = 0; index < bytes.length; index += 1) bytes[index] = Math.floor(Math.random() * 256);
  return Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
}

export function canRetryStream({ attempt = 0, hasDelta = false, stopRequested = false, serverError = false, error = null } = {}) {
  if (attempt >= CLIENT_RETRY_DELAYS_MS.length || hasDelta || stopRequested || serverError || error?.name === "AbortError") return false;
  return error?.retryable === true || error instanceof TypeError || /network|fetch|streaming response/i.test(String(error?.message || ""));
}

export function waitForDelay(delay, signal) {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException("Aborted", "AbortError"));
      return;
    }
    const timer = setTimeout(resolve, delay);
    signal?.addEventListener("abort", () => {
      clearTimeout(timer);
      reject(new DOMException("Aborted", "AbortError"));
    }, { once: true });
  });
}
