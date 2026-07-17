import { esc } from "../core/dom.js";
import { codeLanguageLabel, highlightCode, isHTMLPreviewLanguage } from "./highlight.bundle.js";

let codeBlockSequence = 0;

export function renderMarkdown(markdown, labelsOrCopy = {}, legacyTableLabel = "Markdown table") {
  const labels = normalizeLabels(labelsOrCopy, legacyTableLabel);
  const codeBlocks = [];
  const tableBlocks = [];
  let rest = String(markdown || "").replace(/```([a-zA-Z0-9_-]*)\r?\n([\s\S]*?)```/g, (_, lang, code) => {
    const token = `\u0000CODE${codeBlocks.length}\u0000`;
    codeBlocks.push({ lang, code });
    return token;
  });

  rest = extractTables(rest, tableBlocks);
  rest = renderInline(rest)
    .replace(/^### (.*)$/gm, "<h3>$1</h3>")
    .replace(/^## (.*)$/gm, "<h2>$1</h2>")
    .replace(/^# (.*)$/gm, "<h1>$1</h1>");

  rest = rest.split(/\n{2,}/).map((paragraph) => {
    if (/^<h[1-3]>/.test(paragraph) || paragraph.includes("\u0000CODE") || paragraph.includes("\u0000TABLE")) return paragraph;
    const lines = paragraph.split("\n");
    if (lines.every((line) => /^[-*] /.test(line))) {
      return `<ul>${lines.map((line) => `<li>${line.slice(2)}</li>`).join("")}</ul>`;
    }
    return `<p>${lines.join("<br>")}</p>`;
  }).join("");

  tableBlocks.forEach((table, index) => {
    rest = rest.replace(`\u0000TABLE${index}\u0000`, renderTable(table, labels.tableLabel));
  });
  codeBlocks.forEach((block, index) => {
    rest = rest.replace(`\u0000CODE${index}\u0000`, renderCodeBlock(block, labels));
  });
  return rest;
}

function normalizeLabels(value, tableLabel) {
  if (typeof value === "string") {
    return {
      codeLabel: "Code",
      codeViewsLabel: "Code block views",
      copyLabel: value,
      htmlPreviewLabel: "HTML preview",
      previewLabel: "Preview",
      reloadPreviewLabel: "Reload preview",
      tableLabel,
    };
  }
  return {
    codeLabel: value.codeLabel || "Code",
    codeViewsLabel: value.codeViewsLabel || "Code block views",
    copyLabel: value.copyLabel || "Copy code",
    htmlPreviewLabel: value.htmlPreviewLabel || "HTML preview",
    previewLabel: value.previewLabel || "Preview",
    reloadPreviewLabel: value.reloadPreviewLabel || "Reload preview",
    tableLabel: value.tableLabel || "Markdown table",
  };
}

function renderCodeBlock(block, labels) {
  const source = block.code.replace(/\r?\n$/, "");
  const highlighted = highlightCode(source, block.lang);
  const languageLabel = codeLanguageLabel(block.lang, labels.codeLabel);
  const previewable = isHTMLPreviewLanguage(block.lang);
  const blockID = `chat-code-block-${++codeBlockSequence}`;
  const codePanelID = `${blockID}-code`;
  const previewPanelID = `${blockID}-preview`;
  const tabs = previewable
    ? `<div class="code-block-tabs" role="tablist" aria-label="${esc(labels.codeViewsLabel)}"><button type="button" class="code-block-tab active" role="tab" aria-selected="true" aria-controls="${codePanelID}" data-code-view="code">${esc(labels.codeLabel)}</button><button type="button" class="code-block-tab" role="tab" aria-selected="false" aria-controls="${previewPanelID}" data-code-view="preview">${esc(labels.previewLabel)}</button></div>`
    : "";
  const reload = previewable
    ? `<button type="button" class="code-block-tool hidden" data-reload-preview title="${esc(labels.reloadPreviewLabel)}" aria-label="${esc(labels.reloadPreviewLabel)}"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-refresh"></use></svg></button>`
    : "";
  return `<div class="code-block${previewable ? " has-preview" : ""}" data-code-block data-preview-title="${esc(labels.htmlPreviewLabel)}"><div class="code-block-toolbar"><span class="code-block-language">${esc(languageLabel)}</span>${tabs}<div class="code-block-tools">${reload}<button type="button" class="code-block-tool" data-copy-code title="${esc(labels.copyLabel)}" aria-label="${esc(labels.copyLabel)}"><svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-copy"></use></svg></button></div></div><div id="${codePanelID}" class="code-block-panel" role="tabpanel" data-code-pane="code"><pre><code class="hljs${highlighted.language ? ` language-${highlighted.language}` : ""}">${highlighted.html}</code></pre></div>${previewable ? `<div id="${previewPanelID}" class="html-preview-pane hidden" role="tabpanel" data-code-pane="preview"></div>` : ""}</div>`;
}

function extractTables(text, tables) {
  const lines = text.split(/\r?\n/);
  const output = [];
  for (let index = 0; index < lines.length; index += 1) {
    const header = splitTableRow(lines[index]);
    const delimiter = index + 1 < lines.length ? splitTableRow(lines[index + 1]) : null;
    const alignments = delimiter?.cells.map(tableAlignment) || [];
    const isTable = header?.hasPipe && delimiter?.hasPipe && header.cells.length > 0 &&
      delimiter.cells.length === header.cells.length && alignments.every(Boolean);
    if (!isTable) {
      output.push(lines[index]);
      continue;
    }

    const rows = [];
    let rowIndex = index + 2;
    while (rowIndex < lines.length && lines[rowIndex].trim() !== "") {
      const row = splitTableRow(lines[rowIndex]);
      if (!row?.hasPipe) break;
      rows.push(Array.from({ length: header.cells.length }, (_, column) => row.cells[column] || ""));
      rowIndex += 1;
    }
    const token = `\u0000TABLE${tables.length}\u0000`;
    tables.push({ header: header.cells, alignments, rows });
    if (output.length && output[output.length - 1] !== "") output.push("");
    output.push(token, "");
    index = rowIndex - 1;
  }
  return output.join("\n");
}

export function splitTableRow(line) {
  const source = String(line || "").trim();
  if (!source) return null;
  const cells = [];
  let current = "";
  let hasPipe = false;
  let codeTicks = 0;

  for (let index = 0; index < source.length; index += 1) {
    const character = source[index];
    if (character === "\\" && source[index + 1] === "|") {
      current += "|";
      index += 1;
      continue;
    }
    if (character === "`") {
      let count = 1;
      while (source[index + count] === "`") count += 1;
      const ticks = "`".repeat(count);
      current += ticks;
      if (codeTicks === 0) codeTicks = count;
      else if (codeTicks === count) codeTicks = 0;
      index += count - 1;
      continue;
    }
    if (character === "|" && codeTicks === 0) {
      cells.push(current.trim());
      current = "";
      hasPipe = true;
      continue;
    }
    current += character;
  }
  cells.push(current.trim());
  if (source.startsWith("|") && cells[0] === "") cells.shift();
  if (source.endsWith("|") && cells[cells.length - 1] === "") cells.pop();
  return { cells, hasPipe };
}

function tableAlignment(cell) {
  const value = String(cell || "").trim();
  if (!/^:?-{3,}:?$/.test(value)) return "";
  if (value.startsWith(":") && value.endsWith(":")) return "center";
  if (value.endsWith(":")) return "right";
  return "left";
}

function renderTable(table, label) {
  const header = table.header.map((cell, index) => `<th class="markdown-align-${table.alignments[index]}">${renderInline(cell)}</th>`).join("");
  const body = table.rows.map((row) => `<tr>${row.map((cell, index) => `<td class="markdown-align-${table.alignments[index]}">${renderInline(cell)}</td>`).join("")}</tr>`).join("");
  return `<div class="markdown-table-wrap" role="region" aria-label="${esc(label)}" tabindex="0"><table class="markdown-table"><thead><tr>${header}</tr></thead><tbody>${body}</tbody></table></div>`;
}

function renderInline(value) {
  return esc(value)
    .replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>")
    .replace(/`([^`]+)`/g, "<code>$1</code>")
    .replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>')
    .replace(/(^|\s)(https?:\/\/[^\s<]+)/g, '$1<a href="$2" target="_blank" rel="noopener noreferrer">$2</a>');
}
