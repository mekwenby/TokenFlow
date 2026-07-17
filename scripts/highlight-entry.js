import hljs from "highlight.js/lib/core";
import bash from "highlight.js/lib/languages/bash";
import c from "highlight.js/lib/languages/c";
import cpp from "highlight.js/lib/languages/cpp";
import csharp from "highlight.js/lib/languages/csharp";
import css from "highlight.js/lib/languages/css";
import go from "highlight.js/lib/languages/go";
import java from "highlight.js/lib/languages/java";
import javascript from "highlight.js/lib/languages/javascript";
import json from "highlight.js/lib/languages/json";
import markdown from "highlight.js/lib/languages/markdown";
import python from "highlight.js/lib/languages/python";
import sql from "highlight.js/lib/languages/sql";
import typescript from "highlight.js/lib/languages/typescript";
import xml from "highlight.js/lib/languages/xml";
import yaml from "highlight.js/lib/languages/yaml";

const MAX_HIGHLIGHT_CHARS = 100000;
const CACHE_LIMIT = 64;

const languages = {
  bash,
  c,
  cpp,
  csharp,
  css,
  go,
  java,
  javascript,
  json,
  markdown,
  python,
  sql,
  typescript,
  xml,
  yaml,
};

for (const [name, grammar] of Object.entries(languages)) hljs.registerLanguage(name, grammar);

const aliases = new Map(Object.entries({
  bash: "bash",
  c: "c",
  "c++": "cpp",
  cc: "cpp",
  cpp: "cpp",
  cs: "csharp",
  csharp: "csharp",
  css: "css",
  go: "go",
  golang: "go",
  htm: "xml",
  html: "xml",
  java: "java",
  javascript: "javascript",
  js: "javascript",
  jsx: "javascript",
  cjs: "javascript",
  mjs: "javascript",
  json: "json",
  md: "markdown",
  markdown: "markdown",
  py: "python",
  python: "python",
  sh: "bash",
  shell: "bash",
  sql: "sql",
  ts: "typescript",
  tsx: "typescript",
  typescript: "typescript",
  xhtml: "xml",
  xml: "xml",
  yaml: "yaml",
  yml: "yaml",
}));

const labels = {
  bash: "Shell",
  c: "C",
  cpp: "C++",
  csharp: "C#",
  css: "CSS",
  go: "Go",
  java: "Java",
  javascript: "JavaScript",
  json: "JSON",
  markdown: "Markdown",
  python: "Python",
  sql: "SQL",
  typescript: "TypeScript",
  xml: "HTML/XML",
  yaml: "YAML",
};

const cache = new Map();

export function normalizeCodeLanguage(language) {
  const raw = String(language || "").trim().toLowerCase();
  return aliases.get(raw) || "";
}

export function codeLanguageLabel(language, fallback = "Code") {
  const raw = String(language || "").trim();
  const lowered = raw.toLowerCase();
  if (["html", "htm"].includes(lowered)) return "HTML";
  if (lowered === "xhtml") return "XHTML";
  if (lowered === "xml") return "XML";
  const normalized = normalizeCodeLanguage(language);
  if (normalized) return labels[normalized];
  return raw ? raw.toUpperCase() : fallback;
}

export function isHTMLPreviewLanguage(language) {
  return ["html", "htm", "xhtml"].includes(String(language || "").trim().toLowerCase());
}

export function highlightCode(code, language) {
  const source = String(code || "");
  const normalized = normalizeCodeLanguage(language);
  if (!normalized || source.length > MAX_HIGHLIGHT_CHARS) {
    return { html: escapeHTML(source), highlighted: false, language: normalized };
  }

  const key = `${normalized}\u0000${source}`;
  const cached = cache.get(key);
  if (cached) {
    cache.delete(key);
    cache.set(key, cached);
    return cached;
  }

  let result;
  try {
    result = { html: hljs.highlight(source, { language: normalized, ignoreIllegals: true }).value, highlighted: true, language: normalized };
  } catch {
    result = { html: escapeHTML(source), highlighted: false, language: normalized };
  }
  cache.set(key, result);
  if (cache.size > CACHE_LIMIT) cache.delete(cache.keys().next().value);
  return result;
}

function escapeHTML(value) {
  return String(value).replace(/[&<>"']/g, (character) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  })[character]);
}
