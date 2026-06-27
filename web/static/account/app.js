import { createAPIClient } from "../core/api.js";
import { esc, setRegionLoading, setRegionError, clearRegionBusy, withFormBusy } from "../core/dom.js";
import { date, percent, formatToken, formatCompactNumber } from "../core/format.js";
import { showToast, copyText, iconActionButton, copyButton } from "../core/toast.js";
import { confirmAction } from "../core/confirm.js";
import { initSectionNav } from "../core/nav.js";

// -- I18N & Config -----------------------------------------------------------
const i18n = window.__ACCOUNT_I18N__ || {};
const t = (key) => i18n[key] || key;
const api = createAPIClient({ csrfCookie: "gateway_account_csrf", defaultError: t("request_failed") });

function showError(error) {
  showToast(error?.message || t("request_failed"), "error");
}

// -- State -------------------------------------------------------------------
let keys = [];
let logs = [];
let logsLimit = 50;
let logsOffset = 0;
let logsTotal = 0;
let logsQuery = "";

// -- Render ------------------------------------------------------------------
function renderAddresses() {
  const target = document.querySelector("#account-api-addresses");
  if (!target) return;
  const origin = window.location.origin;
  target.innerHTML = [
    [t("api_base"), origin],
    [t("openai_chat"), `${origin}/v1/chat/completions`],
    [t("openai_models"), `${origin}/v1/models`],
    [t("anthropic_messages"), `${origin}/v1/messages`],
    [t("anthropic_models"), `${origin}/anthropic/v1/models`],
  ].map(([label, value]) => `
    <div class="api-item">
      <div class="api-item-head"><span>${esc(label)}</span>${copyButton(value, t("copy"))}</div>
      <code>${esc(value)}</code>
    </div>`).join("");
}

function renderNewKey(plainKey) {
  const banner = document.querySelector("#account-new-key");
  if (!banner) return;
  banner.innerHTML = `<span>${esc(t("new_key"))}</span><code>${esc(plainKey)}</code>${copyButton(plainKey, t("copy"))}`;
  banner.classList.remove("hidden");
}

function renderKeys() {
  const target = document.querySelector("#account-keys");
  if (!target) return;
  clearRegionBusy(target);
  if (!keys.length) {
    target.innerHTML = `<p class="empty">${esc(t("empty"))}</p>`;
    return;
  }
  const statusPill = (on) => {
    const label = on ? t("enabled") : t("disabled");
    return `<span class="status-pill ${on ? "" : "off"}"><span></span>${esc(label)}</span>`;
  };
  target.innerHTML = `
    <table>
      <thead><tr><th>${t("name")}</th><th>${t("prefix")}</th><th>${t("status")}</th><th>${t("requests")}</th><th>${t("input_tokens")}</th><th>${t("cache_read_tokens")}</th><th>${t("output_tokens")}</th><th>${t("last_used")}</th><th></th></tr></thead>
      <tbody>${keys.map((key) => `
        <tr>
          <td>${esc(key.name)}</td>
          <td><code>${esc(key.prefix)}</code></td>
          <td>${statusPill(key.enabled)}</td>
          <td>${formatCompactNumber(key.request_count)}</td>
          <td>${formatToken(key.input_tokens)}</td>
          <td>${formatToken(key.cache_read_tokens)}</td>
          <td>${formatToken(key.output_tokens)}</td>
          <td>${date(key.last_used_at)}</td>
          <td class="actions">
            ${iconActionButton("edit-account-key", key.id, t("edit"), "edit")}
            ${iconActionButton("reset-account-key", key.id, t("regenerate"), "refresh")}
            ${iconActionButton("delete-account-key", key.id, t("delete"), "trash", "danger")}
          </td>
        </tr>`).join("")}</tbody>
    </table>`;
}

function renderLogs(items = logs) {
  const target = document.querySelector("#account-logs");
  if (!target) return;
  clearRegionBusy(target);
  if (!items.length) {
    target.innerHTML = `<p class="empty">${esc(t("empty"))}</p>`;
    renderLogPager();
    return;
  }
  const rows = items.map((log) => `
    <tr>
      <td>${date(log.created_at)}</td>
      <td>${esc(log.distribution_key_name || "-")}</td>
      <td>${esc(log.model || "-")}</td>
      <td>${log.status_code >= 200 && log.status_code < 300
        ? `<span class="badge">${esc(log.status_code)}</span>`
        : `<span class="badge" style="background:var(--danger-soft);color:var(--danger)">${esc(log.status_code)}</span>`}</td>
      <td>${formatToken(log.input_tokens)}</td>
      <td>${formatToken(log.cache_read_tokens)}</td>
      <td>${percent(log.cache_hit_rate)}</td>
      <td>${formatToken(log.output_tokens)}</td>
      <td>${log.latency_ms != null ? `${log.latency_ms}ms` : "-"}</td>
    </tr>`).join("");
  target.innerHTML = `
    <table>
      <thead><tr><th>${t("time")}</th><th>${t("key")}</th><th>${t("client_model")}</th><th>${t("status")}</th><th>${t("input_tokens")}</th><th>${t("cache_read_tokens")}</th><th>${t("cache_hit_rate")}</th><th>${t("output_tokens")}</th><th>${t("latency")}</th></tr></thead>
      <tbody>${rows}</tbody>
    </table>`;
  renderLogPager();
}

function renderLogPager() {
  const pager = document.querySelector("#account-logs-pager");
  if (!pager) return;
  const start = logsTotal === 0 ? 0 : logsOffset + 1;
  const end = Math.min(logsOffset + logsLimit, logsTotal);
  const prevButton = logsOffset === 0
    ? ""
    : `<button type="button" class="secondary" data-action="account-logs-page" data-id="prev">${esc(t("previous_page"))}</button>`;
  const nextDisabled = logsOffset + logsLimit >= logsTotal ? "disabled" : "";
  pager.innerHTML = `
    <div class="pager">
      <span class="pager-info">${esc(t("showing"))} ${start}-${end} ${esc(t("of"))} ${logsTotal}</span>
      ${prevButton}
      <button type="button" class="secondary" data-action="account-logs-page" data-id="next" ${nextDisabled}>${esc(t("next_page"))}</button>
    </div>`;
}

// -- Key Form ----------------------------------------------------------------
function openKeyForm(key) {
  const form = document.querySelector("#account-key-form");
  if (!form) return;
  form.reset();
  form.elements.id.value = key?.id || "";
  form.elements.name.value = key?.name || "";
  form.elements.enabled.checked = key ? !!key.enabled : true;
  form.querySelector(".account-key-enabled")?.classList.toggle("hidden", !key);
  form.classList.remove("hidden");
  document.querySelector("#account-new-key")?.classList.add("hidden");
}

function closeKeyForm() {
  const form = document.querySelector("#account-key-form");
  if (!form) return;
  form.reset();
  form.classList.add("hidden");
}

// -- Data Loading ------------------------------------------------------------
async function loadKeys(showLoading = true) {
  if (showLoading) setRegionLoading("#account-keys", t("loading"));
  try {
    keys = await api("/account/api/keys");
    renderKeys();
  } catch (error) {
    setRegionError("#account-keys", error.message || t("request_failed"));
    showError(error);
  }
}

async function loadLogs(showLoading = true) {
  if (showLoading) setRegionLoading("#account-logs", t("loading"));
  try {
    const page = await api(`/account/api/logs?limit=${logsLimit}&offset=${logsOffset}&q=${encodeURIComponent(logsQuery)}`);
    logs = page.items || [];
    logsTotal = page.total || 0;
    logsLimit = page.limit || logsLimit;
    logsOffset = page.offset || 0;
    logsQuery = page.query || logsQuery;
    const input = document.querySelector('#account-logs-search-form [name="q"]');
    if (input && input.value !== logsQuery) input.value = logsQuery;
    renderLogs();
  } catch (error) {
    setRegionError("#account-logs", error.message || t("request_failed"));
    showError(error);
  }
}

// -- Action Handlers ---------------------------------------------------------
const actions = {
  "copy": async (target) => {
    await copyText(target.dataset.id || "");
    showToast(t("copied"));
  },
  "open-account-key": () => openKeyForm(null),
  "cancel-account-key": closeKeyForm,
  "clear-account-logs-search": async () => {
    logsQuery = "";
    logsOffset = 0;
    const input = document.querySelector('#account-logs-search-form [name="q"]');
    if (input) input.value = "";
    await loadLogs();
  },
  "edit-account-key": (target) => {
    const key = keys.find((k) => String(k.id) === target.dataset.id);
    if (key) openKeyForm(key);
  },
  "reset-account-key": async (target) => {
    if (!(await confirmAction({ title: t("confirm_title"), message: t("reset_key_confirm"), confirmLabel: t("regenerate"), cancelLabel: t("cancel"), tone: "danger" }))) return;
    const key = await api("/account/api/keys/reset", { method: "POST", body: JSON.stringify({ id: Number(target.dataset.id || 0) }) });
    await loadKeys();
    if (key.plain_key) renderNewKey(key.plain_key);
    showToast(t("saved"));
  },
  "delete-account-key": async (target) => {
    if (!(await confirmAction({ title: t("confirm_title"), message: t("delete_key_confirm"), confirmLabel: t("delete"), cancelLabel: t("cancel"), tone: "danger" }))) return;
    await api(`/account/api/keys?id=${target.dataset.id}`, { method: "DELETE" });
    showToast(t("saved"));
    await loadKeys();
  },
  "account-logs-page": async (target) => {
    logsOffset = target.dataset.id === "prev" ? Math.max(0, logsOffset - logsLimit) : logsOffset + logsLimit;
    await loadLogs();
  },
};

// -- Event Delegation --------------------------------------------------------
document.addEventListener("click", (event) => {
  const target = event.target.closest("[data-action]");
  if (!target) return;
  const handler = actions[target.dataset.action];
  if (handler) {
    Promise.resolve(handler(target)).catch(showError);
  }
});

// -- Form Submission ---------------------------------------------------------
document.querySelector("#account-key-form")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  try {
    await withFormBusy(form, async () => {
      const data = Object.fromEntries(new FormData(form).entries());
      data.id = Number(data.id || 0);
      data.enabled = form.elements.enabled.checked;
      const key = await api("/account/api/keys", { method: data.id ? "PUT" : "POST", body: JSON.stringify(data) });
      closeKeyForm();
      await loadKeys();
      if (key.plain_key) renderNewKey(key.plain_key);
    }, t("saving"));
    showToast(t("saved"));
  } catch (error) {
    showError(error);
  }
});

document.querySelector("#account-logs-search-form")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  logsQuery = String(new FormData(event.currentTarget).get("q") || "").trim();
  logsOffset = 0;
  await loadLogs();
});

// -- Init --------------------------------------------------------------------
renderAddresses();
initSectionNav();
loadKeys().catch(showError);
loadLogs().catch(showError);
