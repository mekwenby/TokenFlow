export const HTML_PREVIEW_CSP = [
  "default-src 'none'",
  "script-src 'unsafe-inline'",
  "style-src 'unsafe-inline' https:",
  "img-src https: data: blob:",
  "font-src https: data:",
  "media-src https: data: blob:",
  "connect-src 'none'",
  "frame-src 'none'",
  "worker-src 'none'",
  "object-src 'none'",
  "base-uri 'none'",
  "form-action 'none'",
].join("; ");

export function buildHTMLPreviewDocument(html) {
  const source = String(html || "");
  const metadata = `<meta http-equiv="Content-Security-Policy" content="${escapeAttribute(HTML_PREVIEW_CSP)}"><meta name="viewport" content="width=device-width, initial-scale=1">`;
  if (/<head(?:\s[^>]*)?>/i.test(source)) return source.replace(/<head(?:\s[^>]*)?>/i, (head) => `${head}${metadata}`);
  if (/<html(?:\s[^>]*)?>/i.test(source)) return source.replace(/<html(?:\s[^>]*)?>/i, (root) => `${root}<head>${metadata}</head>`);
  const fragment = source.replace(/^\s*<!doctype[^>]*>/i, "");
  return `<!doctype html><html><head>${metadata}</head><body>${fragment}</body></html>`;
}

export function setCodeBlockView(block, view, options = {}) {
  if (!block) return;
  const targetView = view === "preview" ? "preview" : "code";
  const codePane = block.querySelector('[data-code-pane="code"]');
  const previewPane = block.querySelector('[data-code-pane="preview"]');
  if (!codePane || (targetView === "preview" && !previewPane)) return;
  for (const tab of block.querySelectorAll("[data-code-view]")) {
    const active = tab.dataset.codeView === targetView;
    tab.classList.toggle("active", active);
    tab.setAttribute("aria-selected", String(active));
    tab.tabIndex = active ? 0 : -1;
    if (active && options.focusTab) tab.focus();
  }
  codePane.hidden = targetView !== "code";
  codePane.classList.toggle("hidden", targetView !== "code");
  if (previewPane) {
    previewPane.hidden = targetView !== "preview";
    previewPane.classList.toggle("hidden", targetView !== "preview");
    if (targetView === "preview" && !previewPane.firstElementChild) loadHTMLPreview(block, options);
    if (targetView === "code") previewPane.replaceChildren();
  }
  block.querySelector("[data-reload-preview]")?.classList.toggle("hidden", targetView !== "preview");
}

export function loadHTMLPreview(block, options = {}) {
  const previewPane = block?.querySelector('[data-code-pane="preview"]');
  const source = block?.querySelector('[data-code-pane="code"] code')?.textContent || "";
  if (!previewPane) return null;
  const frame = block.ownerDocument.createElement("iframe");
  frame.className = "html-preview-frame";
  frame.title = block.dataset.previewTitle || options.previewTitle || "HTML preview";
  frame.setAttribute("sandbox", "allow-scripts");
  frame.setAttribute("referrerpolicy", "no-referrer");
  let initialLoad = true;
  frame.addEventListener("load", () => {
    if (initialLoad) {
      initialLoad = false;
      return;
    }
    if (!frame.isConnected) return;
    setCodeBlockView(block, "code", options);
    options.onNavigationBlocked?.();
  });
  frame.srcdoc = buildHTMLPreviewDocument(source);
  previewPane.replaceChildren(frame);
  return frame;
}

function escapeAttribute(value) {
  return String(value).replace(/[&"]/g, (character) => character === "&" ? "&amp;" : "&quot;");
}
