(function () {
  "use strict";

  function esc(value) {
    return String(value ?? "").replace(/[&<>"']/g, (ch) => ({
      "&": "&amp;",
      "<": "&lt;",
      ">": "&gt;",
      '"': "&quot;",
      "'": "&#39;",
    })[ch]);
  }

  function cookie(name) {
    return document.cookie
      .split("; ")
      .find((row) => row.startsWith(`${name}=`))
      ?.split("=")[1] || "";
  }

  async function api(path, options = {}, config = {}) {
    const headers = { "Content-Type": "application/json", ...(options.headers || {}) };
    if (options.method && options.method !== "GET" && config.csrf) {
      headers["X-CSRF-Token"] = config.csrf;
    }
    const response = await fetch(path, { ...options, headers });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error(body.error || config.defaultError || "Request failed");
    }
    return body;
  }

  function date(value) {
    return value ? new Date(value).toLocaleString() : "";
  }

  function percent(value, fallback = "-") {
    const number = Number(value || 0);
    if (!Number.isFinite(number) || number <= 0) return fallback;
    return `${(number * 100).toFixed(1)}%`;
  }

  function formatCompactNumber(value) {
    const number = Number(value || 0);
    if (!Number.isFinite(number) || number < 1000) return String(Math.max(0, Math.trunc(number || 0)));
    const units = [
      [1000000000, "B"],
      [1000000, "M"],
      [1000, "K"],
    ];
    const [divisor, suffix] = units.find(([base]) => number >= base);
    return `${(number / divisor).toFixed(1).replace(/\.0$/, "")}${suffix}`;
  }

  function formatToken(value) {
    return formatCompactNumber(value);
  }

  function icon(name) {
    return `<svg class="icon" aria-hidden="true"><use href="/admin/static/icons.svg#icon-${name}"></use></svg>`;
  }

  function iconActionButton(action, value, label, iconName, tone = "secondary") {
    return `<button type="button" class="${tone} action-icon" data-${action}="${esc(value)}" title="${esc(label)}" aria-label="${esc(label)}">${icon(iconName)}</button>`;
  }

  function copyButton(value, label, dataName = "copy") {
    return `<button type="button" class="secondary action-icon copy-button" data-${dataName}="${esc(value)}" title="${esc(label)}" aria-label="${esc(label)}">${icon("copy")}</button>`;
  }

  function showToast(message, tone = "success") {
    let toast = document.querySelector("#toast");
    if (!toast) {
      toast = document.createElement("div");
      toast.id = "toast";
      toast.setAttribute("role", "status");
      toast.setAttribute("aria-live", "polite");
      document.body.appendChild(toast);
    }
    toast.className = `toast ${tone === "error" ? "error" : ""}`;
    toast.textContent = message;
    toast.classList.add("visible");
    clearTimeout(showToast.timer);
    showToast.timer = setTimeout(() => toast.classList.remove("visible"), tone === "error" ? 3200 : 1800);
  }

  async function copyText(value) {
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

  function resolveTarget(target) {
    return typeof target === "string" ? document.querySelector(target) : target;
  }

  function loadingHTML(message) {
    return `<p class="empty loading">${esc(message || "Loading...")}</p>`;
  }

  function errorHTML(message) {
    return `<p class="inline-error" role="alert">${esc(message || "Request failed")}</p>`;
  }

  function setRegionLoading(target, message) {
    const element = resolveTarget(target);
    if (!element) return;
    element.setAttribute("aria-busy", "true");
    element.innerHTML = loadingHTML(message);
  }

  function setRegionError(target, message) {
    const element = resolveTarget(target);
    if (!element) return;
    element.removeAttribute("aria-busy");
    element.innerHTML = errorHTML(message);
  }

  function clearRegionBusy(target) {
    const element = resolveTarget(target);
    if (element) element.removeAttribute("aria-busy");
  }

  async function withFormBusy(form, task, busyLabel) {
    const submitButtons = Array.from(form.querySelectorAll('button[type="submit"]'));
    const originals = submitButtons.map((button) => ({
      button,
      text: button.textContent,
      disabled: button.disabled,
    }));
    form.setAttribute("aria-busy", "true");
    submitButtons.forEach((button) => {
      button.disabled = true;
      button.classList.add("busy");
      if (busyLabel) button.textContent = busyLabel;
    });
    try {
      return await task();
    } finally {
      form.removeAttribute("aria-busy");
      originals.forEach(({ button, text, disabled }) => {
        button.disabled = disabled;
        button.classList.remove("busy");
        button.textContent = text;
      });
    }
  }

  let confirmResolve;
  let previousFocus;

  function ensureConfirmDialog() {
    let modal = document.querySelector("#confirm-modal");
    if (modal) return modal;
    modal = document.createElement("div");
    modal.id = "confirm-modal";
    modal.className = "modal hidden";
    modal.setAttribute("role", "dialog");
    modal.setAttribute("aria-modal", "true");
    modal.setAttribute("aria-labelledby", "confirm-title");
    modal.innerHTML = `
      <div class="confirm-dialog">
        <h2 id="confirm-title"></h2>
        <p id="confirm-message"></p>
        <div class="confirm-actions">
          <button type="button" class="secondary" data-confirm-cancel></button>
          <button type="button" data-confirm-ok></button>
        </div>
      </div>`;
    document.body.appendChild(modal);
    modal.addEventListener("click", (event) => {
      if (event.target === modal) closeConfirm(false);
    });
    modal.addEventListener("keydown", (event) => {
      if (modal.classList.contains("hidden")) return;
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        closeConfirm(false);
        return;
      }
      if (event.key !== "Tab") return;
      event.stopPropagation();
      const focusable = Array.from(modal.querySelectorAll("button")).filter((item) => !item.disabled);
      if (!focusable.length) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    });
    modal.querySelector("[data-confirm-cancel]").addEventListener("click", () => closeConfirm(false));
    modal.querySelector("[data-confirm-ok]").addEventListener("click", () => closeConfirm(true));
    return modal;
  }

  function closeConfirm(result) {
    const modal = document.querySelector("#confirm-modal");
    if (!modal || modal.classList.contains("hidden")) return;
    modal.classList.add("hidden");
    modal.setAttribute("aria-hidden", "true");
    if (confirmResolve) confirmResolve(result);
    confirmResolve = undefined;
    if (previousFocus && typeof previousFocus.focus === "function") {
      previousFocus.focus();
    }
    previousFocus = undefined;
  }

  function confirmAction(options = {}) {
    const modal = ensureConfirmDialog();
    previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : undefined;
    modal.querySelector("#confirm-title").textContent = options.title || "Confirm action";
    modal.querySelector("#confirm-message").textContent = options.message || "";
    const cancel = modal.querySelector("[data-confirm-cancel]");
    const ok = modal.querySelector("[data-confirm-ok]");
    cancel.textContent = options.cancelLabel || "Cancel";
    ok.textContent = options.confirmLabel || "Continue";
    ok.className = options.tone === "danger" ? "danger" : "";
    modal.classList.remove("hidden");
    modal.removeAttribute("aria-hidden");
    ok.focus();
    return new Promise((resolve) => {
      confirmResolve = resolve;
    });
  }

  window.TokenFlowUI = {
    api,
    clearRegionBusy,
    confirmAction,
    cookie,
    copyButton,
    copyText,
    date,
    errorHTML,
    esc,
    formatCompactNumber,
    formatToken,
    icon,
    iconActionButton,
    loadingHTML,
    percent,
    setRegionError,
    setRegionLoading,
    showToast,
    withFormBusy,
  };
})();
