import { esc } from "./dom.js";

let toastEl;
let timer;

export function showToast(message, tone = "success") {
  if (!toastEl) {
    toastEl = document.createElement("div");
    toastEl.id = "toast";
    toastEl.setAttribute("role", "status");
    toastEl.setAttribute("aria-live", "polite");
    document.body.appendChild(toastEl);
  }
  toastEl.className = `toast ${tone === "error" ? "error" : ""}`;
  toastEl.textContent = message;
  clearTimeout(timer);
  timer = setTimeout(() => toastEl.classList.remove("visible"), tone === "error" ? 3200 : 1800);
  toastEl.classList.add("visible");
}

export async function copyText(value) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value);
    return;
  }
  const input = document.createElement("textarea");
  input.value = value;
  input.setAttribute("readonly", "");
  input.style.position = "fixed";
  input.style.opacity = "0";
  document.body.appendChild(input);
  input.select();
  document.execCommand("copy");
  input.remove();
}

export function icon(name) {
  return `<svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-${esc(name)}"></use></svg>`;
}

export function iconActionButton(action, value, label, iconName, tone = "secondary") {
  return `<button type="button" class="${tone} action-icon" data-action="${esc(action)}" data-id="${esc(String(value))}" title="${esc(label)}" aria-label="${esc(label)}">${icon(iconName)}</button>`;
}

export function copyButton(value, label = "Copy") {
  return `<button type="button" class="secondary action-icon copy-button" data-action="copy" data-id="${esc(String(value))}" title="${esc(label)}" aria-label="${esc(label)}">${icon("copy")}</button>`;
}
