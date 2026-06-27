import { createAPIClient } from "../core/api.js";
import { Store } from "../core/state.js";
import { esc, cookie, loadingHTML, errorHTML, setRegionLoading, setRegionError, clearRegionBusy, withFormBusy } from "../core/dom.js";
import { date, percent, formatToken, formatCompactNumber } from "../core/format.js";
import { showToast, copyText, icon, iconActionButton, copyButton } from "../core/toast.js";
import { confirmAction } from "../core/confirm.js";
import { initSectionNav } from "../core/nav.js";
import { renderTable, statusPill } from "../components/data-table.js";
import { renderBarChart, renderLineChart } from "../components/chart.js";
import { openForm, closeForm, populateForm, submitForm, confirmDelete } from "../components/crud-manager.js";

// -- I18N & Config -----------------------------------------------------------
const i18n = window.__ADMIN_I18N__ || {};
const t = (key) => i18n[key] || key;
const api = createAPIClient({ csrfCookie: "gateway_csrf", defaultError: t("request_failed") });
const csrf = () => cookie("gateway_csrf");

const tokenUsageSeries = [
  { key: "input_tokens", labelKey: "input_tokens", color: "#1a73e8" },
  { key: "output_tokens", labelKey: "output_tokens", color: "#00897b" },
  { key: "cache_read_tokens", labelKey: "cache_read_tokens", color: "#7e57c2" },
  { key: "cache_creation_tokens", labelKey: "cache_creation_tokens", color: "#f4511e" },
];

// -- State -------------------------------------------------------------------
const state = new Store({
  providers: [], mappings: [], keys: [], users: [],
  logsLimit: 50, logsOffset: 0, logsTotal: 0, logsQuery: "",
  tokenUsageRange: "24h",
});

// -- Helpers -----------------------------------------------------------------
function showError(error) {
  showToast(error?.message || t("request_failed"), "error");
}

function setLoading(selector) { setRegionLoading(selector, t("loading")); }
function setError(selector, error) { setRegionError(selector, error?.message || error || t("request_failed")); }

function formData(form) { return Object.fromEntries(new FormData(form)); }

function splitModels(value) {
  return (value || "").split(/[\n,]+/).map((s) => s.trim()).filter(Boolean);
}

function statusIndicator(on) {
  return statusPill(on, t("enabled"), t("disabled"));
}

function actionButtons(...btns) {
  return btns.map(([action, id, label, iconName, tone]) =>
    iconActionButton(action, id, label, iconName, tone)).join("");
}

function detailScopeLabel(scope) {
  if (scope === "admin") return t("admin");
  if (scope === "user") return t("detail_scope_user");
  return scope === "key" ? t("detail_scope_key") : t("detail_scope_provider");
}

function detailURL(scope, id) {
  return `/admin/api/model-token-details?scope=${encodeURIComponent(scope)}&id=${encodeURIComponent(id)}`;
}

function tokenUsageURL() {
  const offset = -new Date().getTimezoneOffset();
  return `/admin/api/token-usage?range=${encodeURIComponent(state.get("tokenUsageRange"))}&tz_offset=${offset}`;
}

function niceMax(value) {
  if (!Number.isFinite(value) || value <= 0) return 1;
  const exponent = Math.pow(10, Math.floor(Math.log10(value)));
  const normalized = value / exponent;
  const rounded = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10;
  return rounded * exponent;
}

function truncateText(value, maxLen = 24) {
  const text = String(value || "-");
  return text.length > maxLen ? `${text.slice(0, maxLen - 1)}...` : text;
}

// -- Render: API Addresses ---------------------------------------------------
function renderAPIAddresses() {
  const target = document.querySelector("#api-addresses");
  if (!target) return;
  const origin = window.location.origin;
  const items = [
    [t("api_base"), origin],
    [t("openai_chat"), `${origin}/v1/chat/completions`],
    [t("openai_models"), `${origin}/v1/models`],
    [t("anthropic_messages"), `${origin}/v1/messages`],
    [t("anthropic_models"), `${origin}/anthropic/v1/models`],
    [t("legacy_anthropic"), `${origin}/anthropic/v1/messages`],
  ];
  target.innerHTML = items.map(([label, value]) => `
    <div class="api-item">
      <div class="api-item-head"><span>${esc(label)}</span>${copyButton(value, t("copy"))}</div>
      <code>${esc(value)}</code>
    </div>`).join("");
}

// -- Render: Stats -----------------------------------------------------------
function renderStats(stats) {
  const target = document.querySelector("#stats");
  if (!target) return;
  target.innerHTML = [
    [t("requests"), stats.total_requests],
    [t("input_tokens"), formatToken(stats.input_tokens)],
    [t("output_tokens"), formatToken(stats.output_tokens)],
    [t("active_keys"), stats.active_keys],
    [t("active_users"), stats.active_users],
    [t("pending_users"), stats.pending_users],
    [t("providers"), stats.providers],
  ].map(([label, value]) => `<div class="stat"><span>${esc(label)}</span><strong>${esc(value ?? 0)}</strong></div>`).join("");
}

// -- Render: Token Usage Chart -----------------------------------------------
function renderTokenUsage(report) {
  const container = document.querySelector("#token-usage");
  if (!container) return;
  clearRegionBusy(container);
  const range = report?.range || state.get("tokenUsageRange");
  state.set("tokenUsageRange", range);
  updateUsageRangeControls(range);

  const points = Array.isArray(report?.points) ? report.points : [];
  const totals = {};
  for (const s of tokenUsageSeries) {
    totals[s.key] = points.reduce((sum, p) => sum + (Number(p?.[s.key]) || 0), 0);
  }
  const totalTokens = totals.input_tokens + totals.output_tokens;
  const legend = tokenUsageSeries.map((s) => `
    <span class="usage-legend-item">
      <span class="usage-color" style="background:${s.color}"></span>
      <span>${esc(t(s.labelKey))}</span>
      <strong>${formatToken(totals[s.key])}</strong>
    </span>`).join("");
  const summary = `
    <div class="usage-summary">
      <span>${esc(t("total_tokens"))} <strong>${formatToken(totalTokens)}</strong></span>
      <div class="usage-legend">${legend}</div>
    </div>`;

  const hasTokens = Object.values(totals).some((v) => v > 0);
  if (!points.length || !hasTokens) {
    container.innerHTML = `${summary}<div class="usage-empty">${esc(t("token_usage_empty"))}</div>`;
    return;
  }

  const width = 900, height = 300, left = 64, right = 24, top = 18, bot = 238;
  const plotW = width - left - right, plotH = bot - top;
  const maxVal = niceMax(Math.max(...points.flatMap((p) => tokenUsageSeries.map((s) => Number(p?.[s.key]) || 0))));

  const yTicks = [0, 0.25, 0.5, 0.75, 1].map((ratio) => {
    const y = bot - ratio * plotH;
    return `<g>
      <line class="chart-grid" x1="${left}" y1="${y}" x2="${width - right}" y2="${y}"></line>
      <text class="chart-axis" x="${left - 10}" y="${y + 4}" text-anchor="end">${formatToken(maxVal * ratio)}</text>
    </g>`;
  }).join("");

  const xLabels = points.map((p, i) => {
    const show = report.granularity === "day" || i % 4 === 0 || i === points.length - 1;
    if (!show) return "";
    const x = left + (points.length === 1 ? 0 : (plotW * i) / (points.length - 1));
    const label = report.granularity === "day"
      ? new Date(p.bucket_start).toLocaleDateString(undefined, { month: "numeric", day: "numeric" })
      : new Date(p.bucket_start).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
    return `<text class="chart-axis" x="${x}" y="${bot + 32}" text-anchor="middle">${esc(label)}</text>`;
  }).join("");

  const lines = tokenUsageSeries.map((s) => {
    const d = points.map((p, i) => {
      const v = Number(p?.[s.key]) || 0;
      const x = left + (points.length === 1 ? 0 : (plotW * i) / (points.length - 1));
      const y = bot - (v / maxVal) * plotH;
      return `${i === 0 ? "M" : "L"} ${x.toFixed(2)} ${y.toFixed(2)}`;
    }).join(" ");
    return `<path class="chart-line" d="${d}" stroke="${s.color}"></path>`;
  }).join("");

  container.innerHTML = `${summary}
    <svg class="chart-svg" viewBox="0 0 ${width} ${height}" role="img" aria-label="${esc(t("token_usage_chart"))}">
      ${yTicks}${lines}${xLabels}
    </svg>`;
}

function updateUsageRangeControls(range) {
  document.querySelectorAll("[data-action='usage-range']").forEach((btn) => {
    const active = btn.dataset.id === range;
    btn.classList.toggle("active", active);
    btn.setAttribute("aria-pressed", active ? "true" : "false");
  });
}

// -- Render: Entities (using renderTable) ------------------------------------
function renderProviders() {
  const target = document.querySelector("#providers");
  if (!target) return;
  clearRegionBusy(target);
  const providers = state.get("providers");
  renderTable(target, {
    columns: [
      { label: t("name"), render: (p) => `${esc(p.name)} ${p.is_default ? `<span class="badge">${t("default")}</span>` : ""}` },
      { label: t("protocol"), key: "protocol" },
      { label: t("status"), render: (p) => statusIndicator(p.enabled) },
      { label: t("requests"), key: "request_count" },
      { label: t("input_tokens"), render: (p) => formatToken(p.input_tokens) },
      { label: t("output_tokens"), render: (p) => formatToken(p.output_tokens) },
      { label: t("cache_read_tokens"), render: (p) => formatToken(p.cache_read_tokens) },
    ],
    rows: providers,
    emptyText: t("empty"),
    actions: (p) => actionButtons(
      ["detail-provider", p.id, t("details"), "chart"],
      ["edit-provider", p.id, t("edit"), "edit"],
      ["delete-provider", p.id, t("delete"), "trash", "danger"],
    ),
  });
}

function renderMappings() {
  const target = document.querySelector("#mappings");
  if (!target) return;
  clearRegionBusy(target);
  renderTable(target, {
    columns: [
      { label: t("client_model"), key: "client_model" },
      { label: t("provider"), key: "provider_name" },
      { label: t("upstream_model"), key: "upstream_model" },
    ],
    rows: state.get("mappings"),
    emptyText: t("empty"),
    actions: (m) => actionButtons(
      ["edit-mapping", m.id, t("edit"), "edit"],
      ["delete-mapping", m.id, t("delete"), "trash", "danger"],
    ),
  });
}

function renderKeys() {
  const target = document.querySelector("#keys");
  if (!target) return;
  clearRegionBusy(target);
  const keys = state.get("keys");
  if (!keys.length) {
    target.innerHTML = `<p class="empty">${esc(t("empty"))}</p>`;
    return;
  }
  target.innerHTML = `
    <table>
      <thead><tr><th>${t("name")}</th><th>${t("consumer_user")}</th><th>${t("prefix")}</th><th>${t("status")}</th><th>${t("requests")}</th><th>${t("input_tokens")}</th><th>${t("cache_read_tokens")}</th><th>${t("output_tokens")}</th><th>${t("last_used")}</th><th></th></tr></thead>
      <tbody>${keys.map((k) => `
        <tr>
          <td>${esc(k.name)}</td>
          <td>${esc(k.consumer_email || "-")}</td>
          <td><code>${esc(k.prefix)}</code></td>
          <td>${statusIndicator(k.enabled)}</td>
          <td>${k.request_count || 0}</td>
          <td>${formatToken(k.input_tokens)}</td>
          <td>${formatToken(k.cache_read_tokens)}</td>
          <td>${formatToken(k.output_tokens)}</td>
          <td>${date(k.last_used_at)}</td>
          <td class="actions">${actionButtons(
            ["detail-key", k.id, t("details"), "chart"],
            ["edit-key", k.id, t("edit"), "edit"],
            ["reset-key", k.id, t("reset_key"), "refresh"],
            ["reset-key-stats", k.id, t("reset_key_stats"), "reset"],
            ["delete-key", k.id, t("delete"), "trash", "danger"],
          )}</td>
        </tr>`).join("")}</tbody>
    </table>`;
}

function userStatus(status) {
  const labels = {
    active: t("status_active"),
    enabled: t("enabled"),
    pending: t("status_pending"),
    disabled: t("status_disabled"),
  };
  const css = { active: "active", enabled: "active", pending: "pending", disabled: "off" };
  const label = labels[status] || status;
  return `<span class="status-pill ${css[status] || "off"}" title="${esc(label)}" aria-label="${esc(label)}"><span></span>${esc(label)}</span>`;
}

function renderUsers() {
  const target = document.querySelector("#users");
  if (!target) return;
  clearRegionBusy(target);
  renderTable(target, {
    columns: [
      { label: t("email"), key: "email" },
      { label: t("status"), render: (u) => userStatus(u.status) },
      { label: t("quota"), render: (u) => formatToken(u.quota_total_tokens) },
      { label: t("used"), render: (u) => formatToken(u.quota_used_tokens) },
      { label: t("remaining"), render: (u) => formatToken(u.quota_remaining_tokens) },
      { label: t("requests"), render: (u) => formatCompactNumber(u.request_count) },
      { label: t("last_used"), render: (u) => date(u.last_used_at) },
    ],
    rows: state.get("users"),
    emptyText: t("empty"),
    actions: (u) => actionButtons(
      ["detail-user", u.id, t("details"), "chart"],
      ["edit-user", u.id, t("edit"), "edit"],
    ),
  });
}

function renderLogs(items) {
  const target = document.querySelector("#logs");
  if (!target) return;
  target.innerHTML = renderLogTable(items);
  renderLogPager();
}

function renderLogTable(items) {
  if (!items.length) return `<p class="empty">${esc(t("empty"))}</p>`;
  const rows = items.map((l) => `
    <tr>
      <td>${date(l.created_at)}</td>
      <td>${esc(l.distribution_key_name || "-")}</td>
      <td>${esc(l.consumer_email || "-")}</td>
      <td>${esc(l.admin_username || "-")}</td>
      <td>${esc(l.model || "-")}</td>
      <td>${esc(l.provider_name || "-")}</td>
      <td>${esc(l.upstream_model || l.model || "-")}</td>
      <td>${l.status_code >= 200 && l.status_code < 300
        ? `<span class="badge">${esc(l.status_code)}</span>`
        : `<span class="badge" style="background:var(--danger-soft);color:var(--danger)">${esc(l.status_code)}</span>`}</td>
      <td>${formatToken(l.input_tokens)}</td>
      <td>${formatToken(l.cache_read_tokens)}</td>
      <td>${percent(l.cache_hit_rate)}</td>
      <td>${formatToken(l.output_tokens)}</td>
      <td>${l.latency_ms != null ? `${l.latency_ms}ms` : "-"}</td>
    </tr>`).join("");
  return `<div class="table-wrap"><table><thead><tr>
    <th>${t("time")}</th><th>${t("key")}</th><th>${t("consumer_user")}</th><th>${t("admin")}</th><th>${t("client_model")}</th>
    <th>${t("upstream_provider")}</th><th>${t("upstream_model")}</th><th>${t("status")}</th>
    <th>${t("input_tokens")}</th><th>${t("cache_read_tokens")}</th><th>${t("cache_hit_rate")}</th><th>${t("output_tokens")}</th><th>${t("latency")}</th>
  </tr></thead><tbody>${rows}</tbody></table></div>`;
}

function renderLogPager() {
  const pager = document.querySelector("#logs-pager");
  if (!pager) return;
  const offset = state.get("logsOffset");
  const limit = state.get("logsLimit");
  const total = state.get("logsTotal");
  const nextDisabled = offset + limit >= total ? "disabled" : "";
  const prevButton = offset === 0
    ? ""
    : `<button type="button" class="secondary" data-action="logs-page" data-id="prev">${esc(t("previous_page"))}</button>`;
  const start = total === 0 ? 0 : offset + 1;
  const end = Math.min(offset + limit, total);
  pager.innerHTML = `
    <div class="pager">
      <span class="pager-info">${esc(t("showing"))} ${start}-${end} ${esc(t("of"))} ${total}</span>
      ${prevButton}
      <button type="button" class="secondary" data-action="logs-page" data-id="next" ${nextDisabled}>${esc(t("next_page"))}</button>
    </div>`;
}

// -- Model Details -----------------------------------------------------------
function closeModelDetails() {
  document.querySelector("#detail-modal")?.classList.add("hidden");
}

async function openModelDetails(scope, id) {
  const modal = document.querySelector("#detail-modal");
  const titleEl = document.querySelector("#detail-modal-title");
  const subtitle = document.querySelector("#detail-modal-subtitle");
  const body = document.querySelector("#detail-modal-body");
  if (!modal || !titleEl || !subtitle || !body) return;
  titleEl.textContent = t("model_token_details");
  subtitle.textContent = detailScopeLabel(scope);
  body.innerHTML = loadingHTML(t("loading"));
  modal.classList.remove("hidden");
  modal.querySelector("[data-action='close-detail']")?.focus();
  try {
    const report = await api(detailURL(scope, id));
    renderModelDetails(body, subtitle, report);
  } catch (error) {
    body.innerHTML = errorHTML(error.message || t("request_failed"));
    showError(error);
  }
}

function renderModelDetails(body, subtitle, report) {
  const totals = report?.totals || {};
  const items = Array.isArray(report?.items) ? report.items : [];
  if (subtitle) {
    subtitle.textContent = `${detailScopeLabel(report?.scope)} - ${report?.name || "-"}`;
  }
  const summary = `
    <div class="detail-stats">
      ${detailStat(t("requests"), totals.requests || 0)}
      ${detailStat(t("total_tokens"), formatToken(totals.total_tokens))}
      ${detailStat(t("input_tokens"), formatToken(totals.input_tokens))}
      ${detailStat(t("output_tokens"), formatToken(totals.output_tokens))}
      ${detailStat(t("cache_read_tokens"), formatToken(totals.cache_read_tokens))}
      ${detailStat(t("cache_creation_tokens"), formatToken(totals.cache_creation_tokens))}
      ${detailStat(t("cache_hit_rate"), percent(totals.cache_hit_rate))}
    </div>`;
  if (!items.length) {
    body.innerHTML = `${summary}<p class="empty">${esc(t("empty"))}</p>`;
    return;
  }
  const chartHTML = renderBarChart({
    items: items.map((item) => ({
      label: item.model,
      values: Object.fromEntries(tokenUsageSeries.map((s) => [s.key, item[s.key] || 0])),
    })),
    series: tokenUsageSeries.map((s) => ({ key: s.key, color: s.color, label: t(s.labelKey) })),
  });
  body.innerHTML = `${summary}${chartHTML}
    <div class="table-wrap detail-table">
      <table>
        <thead><tr><th>${t("model")}</th><th>${t("requests")}</th><th>${t("total_tokens")}</th><th>${t("input_tokens")}</th><th>${t("output_tokens")}</th><th>${t("cache_read_tokens")}</th><th>${t("cache_creation_tokens")}</th><th>${t("cache_hit_rate")}</th></tr></thead>
        <tbody>${items.map((item) => `
          <tr>
            <td>${esc(item.model || "-")}</td>
            <td>${item.requests || 0}</td>
            <td>${formatToken(item.total_tokens)}</td>
            <td>${formatToken(item.input_tokens)}</td>
            <td>${formatToken(item.output_tokens)}</td>
            <td>${formatToken(item.cache_read_tokens)}</td>
            <td>${formatToken(item.cache_creation_tokens)}</td>
            <td>${percent(item.cache_hit_rate)}</td>
          </tr>`).join("")}</tbody>
      </table>
    </div>`;
}

function detailStat(label, value) {
  return `<div class="detail-stat"><span>${esc(label)}</span><strong>${esc(value)}</strong></div>`;
}

// -- Data Loading ------------------------------------------------------------
async function loadAll() {
  renderAPIAddresses();
  for (const selector of ["#stats", "#token-usage", "#users", "#providers", "#mappings", "#keys"]) {
    setLoading(selector);
  }
  const tasks = [
    { name: "stats", selector: "#stats", promise: api("/admin/api/stats"), onSuccess: renderStats },
    { name: "tokenUsage", selector: "#token-usage", promise: loadTokenUsageData(), onSuccess: renderTokenUsage },
    { name: "providers", selector: "#providers", promise: api("/admin/api/providers"), onSuccess: (data) => { state.set("providers", data || []); renderProviders(); } },
    { name: "mappings", selector: "#mappings", promise: api("/admin/api/model-mappings"), onSuccess: (data) => { state.set("mappings", data || []); renderMappings(); } },
    { name: "keys", selector: "#keys", promise: api("/admin/api/keys"), onSuccess: (data) => { state.set("keys", data || []); renderKeys(); } },
    { name: "users", selector: "#users", promise: api("/admin/api/users"), onSuccess: (data) => { state.set("users", data || []); renderUsers(); } },
  ];
  const results = await Promise.allSettled(tasks.map(async (task) => {
    try {
      const data = await task.promise;
      task.onSuccess(data);
    } catch (error) {
      setError(task.selector, error);
    }
  }));
  // Populate provider dropdown for mapping form
  const sel = document.querySelector("#mapping-form [name=\"provider_id\"]");
  if (sel) {
    const providers = state.get("providers");
    sel.innerHTML = providers.map((p) => `<option value="${p.id}">${esc(p.name)}</option>`).join("");
  }
  await loadLogs();
}

async function loadTokenUsageData() { return api(tokenUsageURL()); }

async function loadLogs(showLoading = true) {
  if (showLoading) setLoading("#logs");
  try {
    const query = encodeURIComponent(state.get("logsQuery") || "");
    const limit = state.get("logsLimit"), offset = state.get("logsOffset");
    const page = await api(`/admin/api/logs?limit=${limit}&offset=${offset}&q=${query}`);
    state.update({ logsTotal: page.total || 0, logsLimit: page.limit || limit, logsOffset: page.offset || 0, logsQuery: page.query || state.get("logsQuery") || "" });
    const input = document.querySelector('#logs-search-form [name="q"]');
    if (input && input.value !== state.get("logsQuery")) input.value = state.get("logsQuery");
    renderLogs(page.items || []);
  } catch (error) {
    setError("#logs", error);
    showError(error);
  }
}

function renderNewKey(plainKey) {
  const banner = document.querySelector("#new-key");
  if (!banner) return;
  banner.innerHTML = `<span>${esc(t("new_key"))}</span><code>${esc(plainKey)}</code>${copyButton(plainKey, t("copy"))}`;
  banner.classList.remove("hidden");
}

// -- Action Handlers (dispatch table) ----------------------------------------
const actions = {
  "close-detail": closeModelDetails,
  "copy": async (target) => {
    await copyText(target.dataset.id || "");
    showToast(t("copied"));
  },
  "clear-logs-search": async () => {
    state.update({ logsQuery: "", logsOffset: 0 });
    const input = document.querySelector('#logs-search-form [name="q"]');
    if (input) input.value = "";
    await loadLogs();
  },
  "usage-range": async (target) => {
    const range = target.dataset.id || "24h";
    state.set("tokenUsageRange", range);
    updateUsageRangeControls(range);
    await loadTokenUsage();
  },
  "detail-provider": (target) => openModelDetails("provider", target.dataset.id),
  "detail-key": (target) => openModelDetails("key", target.dataset.id),
  "detail-user": (target) => openModelDetails("user", target.dataset.id),
  "open": (target) => openForm(target.dataset.id),
  "cancel": (target) => closeForm(target.closest("form")),
  "edit-provider": (target) => {
    const p = state.get("providers").find((x) => String(x.id) === target.dataset.id);
    if (!p) return;
    const form = document.querySelector("#provider-form");
    openForm("provider-form");
    form.elements.id.value = p.id;
    form.elements.name.value = p.name || "";
    form.elements.protocol.value = p.protocol || "";
    form.elements.base_api.value = p.base_api || "";
    form.elements.default_model.value = p.default_model || "";
    form.elements.models.value = (p.models || []).join("\n");
    form.elements.enabled.checked = !!p.enabled;
    form.elements.is_default.checked = !!p.is_default;
  },
  "delete-provider": (target) => confirmDelete({
    message: t("delete_provider_confirm"), confirmLabel: t("delete"),
    apiCall: () => api(`/admin/api/providers?id=${target.dataset.id}`, { method: "DELETE" }),
    onSuccess: loadAll, t,
  }),
  "edit-mapping": (target) => {
    const m = state.get("mappings").find((x) => String(x.id) === target.dataset.id);
    if (!m) return;
    const form = document.querySelector("#mapping-form");
    openForm("mapping-form");
    form.elements.id.value = m.id;
    form.elements.client_model.value = m.client_model || "";
    form.elements.provider_id.value = m.provider_id || "";
    form.elements.upstream_model.value = m.upstream_model || "";
  },
  "delete-mapping": (target) => confirmDelete({
    message: t("delete_mapping_confirm"), confirmLabel: t("delete"),
    apiCall: () => api(`/admin/api/model-mappings?id=${target.dataset.id}`, { method: "DELETE" }),
    onSuccess: loadAll, t,
  }),
  "edit-user": (target) => {
    const u = state.get("users").find((x) => String(x.id) === target.dataset.id);
    if (!u) return;
    const form = document.querySelector("#user-form");
    openForm("user-form");
    form.elements.id.value = u.id;
    form.elements.status.value = u.status || "pending";
    form.elements.quota_total_tokens.value = u.quota_total_tokens || 0;
  },
  "edit-key": (target) => {
    const k = state.get("keys").find((x) => String(x.id) === target.dataset.id);
    if (!k) return;
    const form = document.querySelector("#key-form");
    openForm("key-form");
    form.elements.id.value = k.id;
    form.elements.name.value = k.name;
    form.elements.enabled.checked = !!k.enabled;
    form.querySelector(".key-enabled")?.classList.remove("hidden");
  },
  "delete-key": (target) => confirmDelete({
    message: t("delete_key_confirm"), confirmLabel: t("delete"),
    apiCall: () => api(`/admin/api/keys?id=${target.dataset.id}`, { method: "DELETE" }),
    onSuccess: loadAll, t,
  }),
  "reset-key": async (target) => {
    if (!(await confirmAction({ title: t("confirm_title"), message: t("reset_key_confirm"), confirmLabel: t("reset_key"), cancelLabel: t("cancel"), tone: "danger" }))) return;
    const key = await api("/admin/api/keys/reset", { method: "POST", body: JSON.stringify({ id: Number(target.dataset.id || 0) }) });
    await loadAll();
    if (key.plain_key) renderNewKey(key.plain_key);
    showToast(t("saved"));
  },
  "reset-key-stats": (target) => confirmDelete({
    message: t("reset_key_stats_confirm"), confirmLabel: t("reset_key_stats"),
    apiCall: () => api("/admin/api/keys/reset-stats", { method: "POST", body: JSON.stringify({ id: Number(target.dataset.id || 0) }) }),
    onSuccess: loadAll, t,
  }),
  "logs-page": async (target) => {
    const dir = target.dataset.id;
    const off = state.get("logsOffset"), lim = state.get("logsLimit");
    state.set("logsOffset", dir === "prev" ? Math.max(0, off - lim) : off + lim);
    await loadLogs();
  },
};

// -- Global Event Delegation -------------------------------------------------
document.addEventListener("click", (event) => {
  const target = event.target.closest("[data-action]");
  if (!target) return;
  const action = target.dataset.action;
  const handler = actions[action];
  if (handler) {
    Promise.resolve(handler(target)).catch(showError);
  }
});

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") closeModelDetails();
});

// -- Form Submissions --------------------------------------------------------
document.querySelector("#provider-form")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  await submitForm(form, async () => {
    const data = formData(form);
    data.id = Number(data.id || 0);
    data.models = splitModels(data.models);
    data.enabled = form.elements.enabled.checked;
    data.is_default = form.elements.is_default.checked;
    return api("/admin/api/providers", { method: data.id ? "PUT" : "POST", body: JSON.stringify(data) });
  }, loadAll);
});

document.querySelector("#mapping-form")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  await submitForm(form, async () => {
    const data = formData(form);
    data.id = Number(data.id || 0);
    data.provider_id = Number(data.provider_id || 0);
    return api("/admin/api/model-mappings", { method: data.id ? "PUT" : "POST", body: JSON.stringify(data) });
  }, loadAll);
});

document.querySelector("#user-form")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  await submitForm(form, async () => {
    const data = formData(form);
    data.id = Number(data.id || 0);
    data.quota_total_tokens = Number(data.quota_total_tokens || 0);
    return api("/admin/api/users", { method: "PUT", body: JSON.stringify(data) });
  }, loadAll);
});

document.querySelector("#key-form")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  await submitForm(form, async () => {
    const data = formData(form);
    data.id = Number(data.id || 0);
    data.enabled = form.elements.enabled.checked;
    const key = await api("/admin/api/keys", { method: data.id ? "PUT" : "POST", body: JSON.stringify(data) });
    closeForm(form);
    await loadAll();
    if (key.plain_key) renderNewKey(key.plain_key);
  }, loadAll);
});

document.querySelector("#logs-search-form")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  state.set("logsQuery", String(new FormData(event.currentTarget).get("q") || "").trim());
  state.set("logsOffset", 0);
  await loadLogs();
});

// -- Init --------------------------------------------------------------------
async function loadTokenUsage() {
  setLoading("#token-usage");
  try {
    const usage = await loadTokenUsageData();
    renderTokenUsage(usage);
  } catch (error) {
    setError("#token-usage", error);
    showError(error);
  }
}

initSectionNav();
loadAll().catch(showError);
