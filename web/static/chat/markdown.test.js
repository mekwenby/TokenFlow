import assert from "node:assert/strict";
import test from "node:test";

import { codeLanguageLabel, highlightCode, isHTMLPreviewLanguage, normalizeCodeLanguage } from "./highlight.bundle.js";
import { HTML_PREVIEW_CSP, buildHTMLPreviewDocument } from "./html-preview.js";
import { renderMarkdown, splitTableRow } from "./markdown.js";

test("renders GFM tables with optional outer pipes and alignment", () => {
  const outer = renderMarkdown("| Name | Score | Note |\n| :--- | ---: | :---: |\n| Ada | 10 | Great |", "Copy", "Data table");
  assert.match(outer, /<table class="markdown-table">/);
  assert.match(outer, /<th class="markdown-align-left">Name<\/th>/);
  assert.match(outer, /<th class="markdown-align-right">Score<\/th>/);
  assert.match(outer, /<th class="markdown-align-center">Note<\/th>/);
  assert.match(outer, /aria-label="Data table"/);

  const noOuter = renderMarkdown("Name | Score\n--- | ---:\nAda | 10");
  assert.match(noOuter, /<td class="markdown-align-right">10<\/td>/);
});

test("supports escaped pipes and pipes inside inline code", () => {
  const row = splitTableRow("| A \\| B | `x|y` |");
  assert.deepEqual(row.cells, ["A | B", "`x|y`"]);
  const html = renderMarkdown("| Label | Code |\n| --- | --- |\n| A \\| B | `x|y` |");
  assert.match(html, /<td class="markdown-align-left">A \| B<\/td>/);
  assert.match(html, /<code>x\|y<\/code>/);
});

test("fills missing cells and ignores cells beyond the header", () => {
  const html = renderMarkdown("| A | B |\n| --- | --- |\n| one |\n| two | three | ignored |");
  assert.match(html, /<tr><td class="markdown-align-left">one<\/td><td class="markdown-align-left"><\/td><\/tr>/);
  assert.doesNotMatch(html, /ignored/);
});

test("keeps semantic table sections when there are no body rows", () => {
  const html = renderMarkdown("| A | B |\n| --- | --- |");
  assert.match(html, /<thead><tr>.*<\/tr><\/thead><tbody><\/tbody>/);
});

test("does not parse tables inside fenced code or with invalid delimiters", () => {
  const fenced = renderMarkdown("```md\n| A | B |\n| --- | --- |\n```");
  assert.doesNotMatch(fenced, /markdown-table/);
  assert.match(fenced, /\| A \| B \|/);

  const invalid = renderMarkdown("| A | B |\n| -- | nope |\n| one | two |");
  assert.doesNotMatch(invalid, /markdown-table/);
});

test("keeps raw HTML escaped and rejects unsafe link protocols", () => {
  const html = renderMarkdown("| Value |\n| --- |\n| <script>alert(1)</script> |\n\n[x](javascript:alert(1))");
  assert.match(html, /&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
  assert.doesNotMatch(html, /<script>/);
  assert.doesNotMatch(html, /href="javascript:/);
});

test("preserves existing headings lists links and code blocks", () => {
  const html = renderMarkdown("## Heading\n\n- one\n- two\n\n[site](https://example.com)\n\n```js\nconst ok = true;\n```", "Copy code");
  assert.match(html, /<h2>Heading<\/h2>/);
  assert.match(html, /<ul><li>one<\/li><li>two<\/li><\/ul>/);
  assert.match(html, /href="https:\/\/example.com"/);
  assert.match(html, /data-copy-code title="Copy code"/);
});

test("highlights registered programming languages without auto detection", () => {
  assert.equal(normalizeCodeLanguage("mjs"), "javascript");
  assert.equal(normalizeCodeLanguage("golang"), "go");
  assert.equal(normalizeCodeLanguage("unknown"), "");
  assert.equal(codeLanguageLabel("ts"), "TypeScript");
  assert.equal(codeLanguageLabel("html"), "HTML");
  assert.equal(codeLanguageLabel("xml"), "XML");
  const highlighted = highlightCode("const value = 1;", "js");
  assert.equal(highlighted.highlighted, true);
  assert.match(highlighted.html, /hljs-keyword/);
  const plain = highlightCode("<script>alert(1)</script>", "unknown");
  assert.equal(plain.highlighted, false);
  assert.match(plain.html, /&lt;script&gt;/);
});

test("falls back to escaped plain text for oversized code", () => {
  const result = highlightCode(`<tag>${"x".repeat(100000)}`, "html");
  assert.equal(result.highlighted, false);
  assert.match(result.html, /^&lt;tag&gt;/);
});

test("only HTML fences receive accessible code and preview tabs", () => {
  assert.equal(isHTMLPreviewLanguage("xhtml"), true);
  assert.equal(isHTMLPreviewLanguage("xml"), false);
  const html = renderMarkdown("```html\n<h1>Hello</h1>\n```", {
    codeLabel: "Code",
    codeViewsLabel: "Code views",
    copyLabel: "Copy code",
    htmlPreviewLabel: "HTML preview",
    previewLabel: "Preview",
    reloadPreviewLabel: "Reload preview",
  });
  assert.match(html, /class="code-block has-preview"/);
  assert.match(html, /role="tablist" aria-label="Code views"/);
  assert.match(html, /data-code-view="preview"/);
  assert.match(html, /data-preview-title="HTML preview"/);
  assert.match(html, /hljs-tag/);
  const javascript = renderMarkdown("```js\nconst ok = true;\n```");
  assert.doesNotMatch(javascript, /data-code-view="preview"/);
});

test("builds preview documents with a restrictive policy", () => {
  for (const expected of ["connect-src 'none'", "frame-src 'none'", "form-action 'none'", "script-src 'unsafe-inline'", "img-src https: data: blob:"]) {
    assert.match(HTML_PREVIEW_CSP, new RegExp(expected.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  }
  const complete = buildHTMLPreviewDocument("<!doctype html><html><head><title>Demo</title></head><body>OK</body></html>");
  assert.match(complete, /<head><meta http-equiv="Content-Security-Policy"/);
  assert.match(complete, /<title>Demo<\/title>/);
  const fragment = buildHTMLPreviewDocument("<main>Demo</main>");
  assert.match(fragment, /^<!doctype html><html><head>/);
  assert.match(fragment, /<body><main>Demo<\/main><\/body>/);
});
