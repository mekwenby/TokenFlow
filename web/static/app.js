const state = {
  providers: [],
  mappings: [],
  keys: [],
  logsLimit: 50,
  logsOffset: 0,
  logsTotal: 0,
  logsQuery: "",
  tokenUsageRange: "24h",
};

const i18n = window.__ADMIN_I18N__ || {};
const t = (key) => i18n[key] || key;
const csrf = () => document.cookie.split("; ").find((row) => row.startsWith("gateway_csrf="))?.split("=")[1] || "";
const tokenUsageSeries = [
  { key: "input_tokens", labelKey: "input_tokens", color: "#2563eb" },
  { key: "output_tokens", labelKey: "output_tokens", color: "#16a34a" },
  { key: "cache_read_tokens", labelKey: "cache_read_tokens", color: "#9333ea" },
  { key: "cache_creation_tokens", labelKey: "cache_creation_tokens", color: "#ea580c" },
];

async function api(path, options = {}) {
  const headers = { "Content-Type": "application/json", ...(options.headers || {}) };
  if (options.method && options.method !== "GET") {
    headers["X-CSRF-Token"] = csrf();
  }
  const response = await fetch(path, { ...options, headers });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(body.error || t("request_failed"));
  }
  return body;
}

function esc(value) {
  return String(value ?? "").replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  })[ch]);
}

function date(value) {
  if (!value) return "";
  return new Date(value).toLocaleString();
}

function percent(value, fallback = "-") {
  const number = Number(value || 0);
  if (!Number.isFinite(number) || number <= 0) return fallback;
  return `${(number * 100).toFixed(1)}%`;
}

function formatToken(value) {
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

function badge(on, yes = t("enabled"), no = t("disabled")) {
  return `<span class="badge ${on ? "" : "off"}">${on ? yes : no}</span>`;
}

function statusIndicator(on) {
  const label = on ? t("enabled") : t("disabled");
  return `<span class="status-pill ${on ? "" : "off"}" title="${esc(label)}" aria-label="${esc(label)}"><span></span>${esc(label)}</span>`;
}

function iconActionButton(action, value, label, emoji, tone = "secondary") {
  return `<button class="${tone} action-icon" data-${action}="${esc(value)}" title="${esc(label)}" aria-label="${esc(label)}">${emoji}</button>`;
}

async function loadAll() {
  const [stats, tokenUsage, providers, mappings, keys] = await Promise.all([
    api("/admin/api/stats"),
    fetchTokenUsage(),
    api("/admin/api/providers"),
    api("/admin/api/model-mappings"),
    api("/admin/api/keys"),
  ]);
  state.providers = providers || [];
  state.mappings = mappings || [];
  state.keys = keys || [];
  renderStats(stats);
  renderTokenUsage(tokenUsage);
  renderAPIAddresses();
  renderProviders();
  renderMappings();
  renderKeys();
  fillProviderOptions();
  await loadLogs();
}

async function loadLogs() {
  const query = encodeURIComponent(state.logsQuery || "");
  const page = await api(`/admin/api/logs?limit=${state.logsLimit}&offset=${state.logsOffset}&q=${query}`);
  state.logsTotal = page.total || 0;
  state.logsLimit = page.limit || state.logsLimit;
  state.logsOffset = page.offset || 0;
  state.logsQuery = page.query || state.logsQuery || "";
  const input = document.querySelector('#logs-search-form [name="q"]');
  if (input && input.value !== state.logsQuery) input.value = state.logsQuery;
  renderLogs(page.items || []);
}

function renderAPIAddresses() {
  const origin = window.location.origin;
  document.querySelector("#api-addresses").innerHTML = [
    [t("api_base"), origin],
    [t("openai_chat"), `${origin}/v1/chat/completions`],
    [t("openai_models"), `${origin}/v1/models`],
    [t("anthropic_messages"), `${origin}/v1/messages`],
    [t("anthropic_models"), `${origin}/anthropic/v1/models`],
    [t("legacy_anthropic"), `${origin}/anthropic/v1/messages`],
  ].map(([label, value]) => `<div class="api-item"><span>${label}</span><code>${esc(value)}</code></div>`).join("");
}

function renderStats(stats) {
  document.querySelector("#stats").innerHTML = [
    [t("requests"), stats.total_requests],
    [t("input_tokens"), formatToken(stats.input_tokens)],
    [t("output_tokens"), formatToken(stats.output_tokens)],
    [t("active_keys"), stats.active_keys],
    [t("providers"), stats.providers],
  ].map(([label, value]) => `<div class="stat"><span>${label}</span><strong>${value ?? 0}</strong></div>`).join("");
}

function detailScopeLabel(scope) {
  return scope === "key" ? t("detail_scope_key") : t("detail_scope_provider");
}

function detailURL(scope, id) {
  return `/admin/api/model-token-details?scope=${encodeURIComponent(scope)}&id=${encodeURIComponent(id)}`;
}

function closeModelDetails() {
  document.querySelector("#detail-modal")?.classList.add("hidden");
}

async function openModelDetails(scope, id) {
  const modal = document.querySelector("#detail-modal");
  const title = document.querySelector("#detail-modal-title");
  const subtitle = document.querySelector("#detail-modal-subtitle");
  const body = document.querySelector("#detail-modal-body");
  if (!modal || !title || !subtitle || !body) return;
  title.textContent = t("model_token_details");
  subtitle.textContent = detailScopeLabel(scope);
  body.innerHTML = `<p class="empty">${t("loading")}</p>`;
  modal.classList.remove("hidden");
  modal.querySelector("[data-close-detail]")?.focus();
  try {
    const report = await api(detailURL(scope, id));
    renderModelDetails(report);
  } catch (error) {
    closeModelDetails();
    alert(error.message);
  }
}

function detailStat(label, value) {
  return `<div class="detail-stat"><span>${label}</span><strong>${value}</strong></div>`;
}

function truncateText(value, maxLength = 24) {
  const text = String(value || "-");
  return text.length > maxLength ? `${text.slice(0, maxLength - 1)}...` : text;
}

function renderModelDetailChart(items) {
  if (!items.length) return "";
  const width = 900;
  const left = 190;
  const right = 82;
  const top = 28;
  const rowHeight = 70;
  const barHeight = 8;
  const seriesGap = 12;
  const height = top + items.length * rowHeight + 16;
  const plotWidth = width - left - right;
  const maxValue = niceMax(Math.max(...items.flatMap((item) => tokenUsageSeries.map((series) => pointValue(item, series.key)))));
  const legend = tokenUsageSeries.map((series) => `
    <span class="usage-legend-item">
      <span class="usage-color" style="background:${series.color}"></span>
      <span>${t(series.labelKey)}</span>
    </span>`).join("");
  const rows = items.map((item, itemIndex) => {
    const groupTop = top + itemIndex * rowHeight;
    const label = truncateText(item.model, 26);
    const bars = tokenUsageSeries.map((series, seriesIndex) => {
      const value = pointValue(item, series.key);
      const y = groupTop + 8 + seriesIndex * seriesGap;
      const barWidth = value > 0 ? Math.max(2, (value / maxValue) * plotWidth) : 0;
      return `
        <rect class="detail-chart-bar-bg" x="${left}" y="${y}" width="${plotWidth}" height="${barHeight}" rx="4"></rect>
        <rect class="detail-chart-bar" x="${left}" y="${y}" width="${barWidth.toFixed(2)}" height="${barHeight}" rx="4" fill="${series.color}"></rect>
        <text class="chart-axis" x="${left + barWidth + 8}" y="${y + 8}">${formatToken(value)}</text>
        <title>${esc(`${item.model || "-"} ${t(series.labelKey)}: ${formatToken(value)}`)}</title>`;
    }).join("");
    return `
      <g>
        <text class="detail-chart-label" x="${left - 12}" y="${groupTop + 32}" text-anchor="end">${esc(label)}</text>
        ${bars}
      </g>`;
  }).join("");
  return `
    <div class="detail-chart-panel">
      <div class="usage-summary">
        <span>${t("model_token_chart")}</span>
        <div class="usage-legend">${legend}</div>
      </div>
      <div class="detail-chart-scroll">
        <svg class="detail-chart-svg" viewBox="0 0 ${width} ${height}" role="img" aria-label="${esc(t("model_token_chart"))}">
          ${rows}
        </svg>
      </div>
    </div>`;
}

function renderModelDetails(report) {
  const subtitle = document.querySelector("#detail-modal-subtitle");
  const body = document.querySelector("#detail-modal-body");
  if (!body) return;
  const totals = report?.totals || {};
  const items = Array.isArray(report?.items) ? report.items : [];
  if (subtitle) {
    subtitle.textContent = `${detailScopeLabel(report?.scope)} · ${report?.name || "-"}`;
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
    body.innerHTML = `${summary}<p class="empty">${t("empty")}</p>`;
    return;
  }
  body.innerHTML = `
    ${summary}
    ${renderModelDetailChart(items)}
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

function tokenUsageURL() {
  const offset = -new Date().getTimezoneOffset();
  return `/admin/api/token-usage?range=${encodeURIComponent(state.tokenUsageRange)}&tz_offset=${offset}`;
}

async function fetchTokenUsage() {
  return api(tokenUsageURL());
}

async function loadTokenUsage() {
  const usage = await fetchTokenUsage();
  renderTokenUsage(usage);
}

function updateUsageRangeControls(range) {
  document.querySelectorAll("[data-usage-range]").forEach((button) => {
    const active = button.dataset.usageRange === range;
    button.classList.toggle("active", active);
    button.setAttribute("aria-pressed", active ? "true" : "false");
  });
}

function niceMax(value) {
  if (!Number.isFinite(value) || value <= 0) return 1;
  const exponent = Math.pow(10, Math.floor(Math.log10(value)));
  const normalized = value / exponent;
  const rounded = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10;
  return rounded * exponent;
}

function pointValue(point, key) {
  const value = Number(point?.[key] || 0);
  return Number.isFinite(value) && value > 0 ? value : 0;
}

function bucketLabel(value, granularity) {
  const bucket = new Date(value);
  if (granularity === "day") {
    return bucket.toLocaleDateString(undefined, { month: "numeric", day: "numeric" });
  }
  return bucket.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

function usageLinePath(points, key, maxValue, left, bottom, plotWidth, plotHeight) {
  return points.map((point, index) => {
    const x = left + (points.length === 1 ? 0 : (plotWidth * index) / (points.length - 1));
    const y = bottom - (pointValue(point, key) / maxValue) * plotHeight;
    return `${index === 0 ? "M" : "L"} ${x.toFixed(2)} ${y.toFixed(2)}`;
  }).join(" ");
}

function renderTokenUsage(report) {
  const container = document.querySelector("#token-usage");
  if (!container) return;
  const range = report?.range || state.tokenUsageRange;
  state.tokenUsageRange = range;
  updateUsageRangeControls(range);

  const points = Array.isArray(report?.points) ? report.points : [];
  const totals = Object.fromEntries(tokenUsageSeries.map((series) => [
    series.key,
    points.reduce((sum, point) => sum + pointValue(point, series.key), 0),
  ]));
  const totalTokens = totals.input_tokens + totals.output_tokens;
  const legend = tokenUsageSeries.map((series) => `
    <span class="usage-legend-item">
      <span class="usage-color" style="background:${series.color}"></span>
      <span>${t(series.labelKey)}</span>
      <strong>${formatToken(totals[series.key])}</strong>
    </span>`).join("");
  const summary = `
    <div class="usage-summary">
      <span>${t("total_tokens")} <strong>${formatToken(totalTokens)}</strong></span>
      <div class="usage-legend">${legend}</div>
    </div>`;

  const hasTokens = Object.values(totals).some((value) => value > 0);
  if (!points.length || !hasTokens) {
    container.innerHTML = `${summary}<div class="usage-empty">${t("token_usage_empty")}</div>`;
    return;
  }

  const width = 900;
  const height = 300;
  const left = 64;
  const right = 24;
  const top = 18;
  const bottom = 238;
  const plotWidth = width - left - right;
  const plotHeight = bottom - top;
  const maxValue = niceMax(Math.max(...points.flatMap((point) => tokenUsageSeries.map((series) => pointValue(point, series.key)))));
  const yTicks = [0, 0.25, 0.5, 0.75, 1].map((ratio) => {
    const y = bottom - ratio * plotHeight;
    const value = maxValue * ratio;
    return `
      <g>
        <line class="chart-grid" x1="${left}" y1="${y}" x2="${width - right}" y2="${y}"></line>
        <text class="chart-axis" x="${left - 10}" y="${y + 4}" text-anchor="end">${formatToken(value)}</text>
      </g>`;
  }).join("");
  const xLabels = points.map((point, index) => {
    const show = report.granularity === "day" || index % 4 === 0 || index === points.length - 1;
    if (!show) return "";
    const x = left + (plotWidth * index) / (points.length - 1);
    return `<text class="chart-axis" x="${x}" y="${bottom + 32}" text-anchor="middle">${esc(bucketLabel(point.bucket_start, report.granularity))}</text>`;
  }).join("");
  const lines = tokenUsageSeries.map((series) => `
    <path class="chart-line" d="${usageLinePath(points, series.key, maxValue, left, bottom, plotWidth, plotHeight)}" stroke="${series.color}"></path>`).join("");
  const dots = tokenUsageSeries.map((series) => points.map((point, index) => {
    const value = pointValue(point, series.key);
    if (!value) return "";
    const x = left + (plotWidth * index) / (points.length - 1);
    const y = bottom - (value / maxValue) * plotHeight;
    return `<circle class="chart-dot" cx="${x.toFixed(2)}" cy="${y.toFixed(2)}" r="3.2" fill="${series.color}"><title>${esc(`${bucketLabel(point.bucket_start, report.granularity)} ${t(series.labelKey)}: ${formatToken(value)}`)}</title></circle>`;
  }).join("")).join("");

  container.innerHTML = `
    ${summary}
    <svg class="chart-svg" viewBox="0 0 ${width} ${height}" role="img" aria-label="${esc(t("token_usage"))}">
      ${yTicks}
      <line class="chart-axis-line" x1="${left}" y1="${bottom}" x2="${width - right}" y2="${bottom}"></line>
      ${xLabels}
      ${lines}
      ${dots}
    </svg>`;
}

function renderProviders() {
  if (!state.providers.length) {
    document.querySelector("#providers").innerHTML = `<p class="empty">${t("empty")}</p>`;
    return;
  }
  document.querySelector("#providers").innerHTML = `
    <table>
      <thead><tr><th>${t("name")}</th><th>${t("protocol")}</th><th>${t("status")}</th><th>${t("requests")}</th><th>${t("input_tokens")}</th><th>${t("output_tokens")}</th><th>${t("cache_read_tokens")}</th><th></th></tr></thead>
      <tbody>${state.providers.map((p) => `
        <tr>
          <td>${esc(p.name)} ${p.is_default ? `<span class="badge">${t("default")}</span>` : ""}</td>
          <td>${esc(p.protocol)}</td>
          <td>${statusIndicator(p.enabled)}</td>
          <td>${p.request_count || 0}</td>
          <td>${formatToken(p.input_tokens)}</td>
          <td>${formatToken(p.output_tokens)}</td>
          <td>${formatToken(p.cache_read_tokens)}</td>
          <td class="actions">
            ${iconActionButton("detail-provider", p.id, t("details"), "📊")}
            ${iconActionButton("edit-provider", p.id, t("edit"), "✏️")}
            ${iconActionButton("delete-provider", p.id, t("delete"), "🗑️", "danger")}
          </td>
        </tr>`).join("")}</tbody>
    </table>`;
}

function renderMappings() {
  if (!state.mappings.length) {
    document.querySelector("#mappings").innerHTML = `<p class="empty">${t("empty")}</p>`;
    return;
  }
  document.querySelector("#mappings").innerHTML = `
    <table>
      <thead><tr><th>${t("client_model")}</th><th>${t("provider")}</th><th>${t("upstream_model")}</th><th></th></tr></thead>
      <tbody>${state.mappings.map((m) => `
        <tr>
          <td>${esc(m.client_model)}</td>
          <td>${esc(m.provider_name)}</td>
          <td>${esc(m.upstream_model)}</td>
          <td class="actions">
            ${iconActionButton("edit-mapping", m.id, t("edit"), "✏️")}
            ${iconActionButton("delete-mapping", m.id, t("delete"), "🗑️", "danger")}
          </td>
        </tr>`).join("")}</tbody>
    </table>`;
}

function renderKeys() {
  if (!state.keys.length) {
    document.querySelector("#keys").innerHTML = `<p class="empty">${t("empty")}</p>`;
    return;
  }
  document.querySelector("#keys").innerHTML = `
    <table>
      <thead><tr><th>${t("name")}</th><th>${t("prefix")}</th><th>${t("status")}</th><th>${t("requests")}</th><th>${t("input_tokens")}</th><th>${t("cache_read_tokens")}</th><th>${t("output_tokens")}</th><th>${t("last_used")}</th><th></th></tr></thead>
      <tbody>${state.keys.map((k) => `
        <tr>
          <td>${esc(k.name)}</td>
          <td><code>${esc(k.prefix)}</code></td>
          <td>${statusIndicator(k.enabled)}</td>
          <td>${k.request_count || 0}</td>
          <td>${formatToken(k.input_tokens)}</td>
          <td>${formatToken(k.cache_read_tokens)}</td>
          <td>${formatToken(k.output_tokens)}</td>
          <td>${date(k.last_used_at)}</td>
          <td class="actions">
            ${iconActionButton("detail-key", k.id, t("details"), "📊")}
            ${iconActionButton("edit-key", k.id, t("edit"), "✏️")}
            ${iconActionButton("reset-key", k.id, t("reset_key"), "🔄")}
            ${iconActionButton("reset-key-stats", k.id, t("reset_key_stats"), "🧹")}
            ${iconActionButton("delete-key", k.id, t("delete"), "🗑️", "danger")}
          </td>
        </tr>`).join("")}</tbody>
    </table>`;
}

function renderLogPager(rowCount) {
  const start = state.logsTotal === 0 ? 0 : state.logsOffset + 1;
  const end = Math.min(state.logsOffset + rowCount, state.logsTotal);
  const previousDisabled = state.logsOffset <= 0 ? "disabled" : "";
  const nextDisabled = state.logsOffset + state.logsLimit >= state.logsTotal ? "disabled" : "";
  return `
    <div class="pager">
      <span>${start}-${end} / ${state.logsTotal}</span>
      <div class="pager-actions">
        <button class="secondary" data-logs-page="prev" ${previousDisabled}>${t("previous_page")}</button>
        <button class="secondary" data-logs-page="next" ${nextDisabled}>${t("next_page")}</button>
      </div>
    </div>`;
}

function renderLogs(logs) {
  const pager = renderLogPager(logs.length);
  if (!logs.length) {
    document.querySelector("#logs").innerHTML = `${pager}<p class="empty">${t("empty")}</p>`;
    return;
  }
  document.querySelector("#logs").innerHTML = `
    ${pager}
    <table>
      <thead><tr><th>${t("time")}</th><th>${t("protocol")}</th><th>${t("model")}</th><th>${t("provider")}</th><th>${t("distribution_key")}</th><th>${t("status")}</th><th>${t("latency")}</th><th>${t("stream")}</th><th>${t("input_tokens")}</th><th>${t("cache_read_tokens")}</th><th>${t("output_tokens")}</th><th>${t("cache_hit_rate")}</th></tr></thead>
      <tbody>${logs.map((log) => `
        <tr>
          <td>${date(log.created_at)}</td>
          <td>${esc(log.protocol)}</td>
          <td>${esc(log.model)}</td>
          <td>${esc(log.provider_name)}</td>
          <td>${esc(log.distribution_key_name || "-")}</td>
          <td>${log.status_code}</td>
          <td>${log.latency_ms} ms</td>
          <td>${log.stream ? t("yes") : t("no")}</td>
          <td>${formatToken(log.input_tokens)}</td>
          <td>${formatToken(log.cache_read_tokens)}</td>
          <td>${formatToken(log.output_tokens)}</td>
          <td title="${esc(`${t("cache_read_tokens")}: ${formatToken(log.cache_read_tokens)}; ${t("cache_creation_tokens")}: ${formatToken(log.cache_creation_tokens)}`)}">${log.input_tokens ? percent(log.cache_hit_rate) : "-"}</td>
        </tr>`).join("")}</tbody>
    </table>`;
}

function fillProviderOptions() {
  const select = document.querySelector('#mapping-form [name="provider_id"]');
  if (!state.providers.length) {
    select.innerHTML = `<option value="">${t("provider_select_empty")}</option>`;
    return;
  }
  select.innerHTML = state.providers.map((p) => `<option value="${p.id}">${esc(p.name)}</option>`).join("");
}

function openForm(id) {
  const form = document.getElementById(id);
  form.reset();
  form.querySelector('[name="id"]').value = "";
  form.classList.remove("hidden");
  if (id === "key-form") {
    form.querySelector(".key-enabled").classList.add("hidden");
  }
}

function closeForm(form) {
  form.reset();
  form.classList.add("hidden");
}

function formData(form) {
  return Object.fromEntries(new FormData(form).entries());
}

function splitModels(value) {
  return String(value || "")
    .split(/[\n,\t]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

document.addEventListener("click", async (event) => {
  const target = event.target;
  if (target.matches("[data-close-detail]") || target.id === "detail-modal") {
    closeModelDetails();
    return;
  }
  if (target.matches("[data-clear-logs-search]")) {
    state.logsQuery = "";
    state.logsOffset = 0;
    const input = document.querySelector('#logs-search-form [name="q"]');
    if (input) input.value = "";
    await loadLogs();
    return;
  }
  if (target.matches("[data-usage-range]")) {
    state.tokenUsageRange = target.dataset.usageRange || "24h";
    updateUsageRangeControls(state.tokenUsageRange);
    await loadTokenUsage();
    return;
  }
  if (target.matches("[data-detail-provider]")) {
    await openModelDetails("provider", target.dataset.detailProvider);
    return;
  }
  if (target.matches("[data-detail-key]")) {
    await openModelDetails("key", target.dataset.detailKey);
    return;
  }
  if (target.matches("[data-open]")) {
    openForm(target.dataset.open);
  }
  if (target.matches("[data-cancel]")) {
    closeForm(target.closest("form"));
  }
  if (target.matches("[data-edit-provider]")) {
    const provider = state.providers.find((p) => String(p.id) === target.dataset.editProvider);
    const form = document.querySelector("#provider-form");
    openForm("provider-form");
    for (const key of ["id", "name", "protocol", "base_api", "default_model"]) form.elements[key].value = provider[key] || "";
    form.elements.models.value = (provider.models || []).join("\n");
    form.elements.enabled.checked = !!provider.enabled;
    form.elements.is_default.checked = !!provider.is_default;
  }
  if (target.matches("[data-delete-provider]") && confirm(t("delete_provider_confirm"))) {
    await api(`/admin/api/providers?id=${target.dataset.deleteProvider}`, { method: "DELETE" });
    await loadAll();
  }
  if (target.matches("[data-edit-mapping]")) {
    const mapping = state.mappings.find((m) => String(m.id) === target.dataset.editMapping);
    const form = document.querySelector("#mapping-form");
    openForm("mapping-form");
    for (const key of ["id", "client_model", "provider_id", "upstream_model"]) form.elements[key].value = mapping[key] || "";
  }
  if (target.matches("[data-delete-mapping]") && confirm(t("delete_mapping_confirm"))) {
    await api(`/admin/api/model-mappings?id=${target.dataset.deleteMapping}`, { method: "DELETE" });
    await loadAll();
  }
  if (target.matches("[data-edit-key]")) {
    const key = state.keys.find((k) => String(k.id) === target.dataset.editKey);
    const form = document.querySelector("#key-form");
    openForm("key-form");
    form.elements.id.value = key.id;
    form.elements.name.value = key.name;
    form.elements.enabled.checked = !!key.enabled;
    form.querySelector(".key-enabled").classList.remove("hidden");
  }
  if (target.matches("[data-delete-key]") && confirm(t("delete_key_confirm"))) {
    await api(`/admin/api/keys?id=${target.dataset.deleteKey}`, { method: "DELETE" });
    await loadAll();
  }
  if (target.matches("[data-reset-key]") && confirm(t("reset_key_confirm"))) {
    const key = await api("/admin/api/keys/reset", { method: "POST", body: JSON.stringify({ id: Number(target.dataset.resetKey || 0) }) });
    if (key.plain_key) {
      const banner = document.querySelector("#new-key");
      banner.textContent = `${t("new_key")}: ${key.plain_key}`;
      banner.classList.remove("hidden");
    }
    await loadAll();
  }
  if (target.matches("[data-reset-key-stats]") && confirm(t("reset_key_stats_confirm"))) {
    await api("/admin/api/keys/reset-stats", { method: "POST", body: JSON.stringify({ id: Number(target.dataset.resetKeyStats || 0) }) });
    await loadAll();
  }
  if (target.matches("[data-logs-page]")) {
    if (target.dataset.logsPage === "prev") {
      state.logsOffset = Math.max(0, state.logsOffset - state.logsLimit);
    } else {
      state.logsOffset += state.logsLimit;
    }
    await loadLogs();
  }
});

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    closeModelDetails();
  }
});

document.querySelector("#logs-search-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  state.logsQuery = String(new FormData(event.currentTarget).get("q") || "").trim();
  state.logsOffset = 0;
  await loadLogs();
});

document.querySelector("#provider-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  const data = formData(form);
  data.id = Number(data.id || 0);
  data.models = splitModels(data.models);
  data.enabled = form.elements.enabled.checked;
  data.is_default = form.elements.is_default.checked;
  const method = data.id ? "PUT" : "POST";
  await api("/admin/api/providers", { method, body: JSON.stringify(data) });
  closeForm(form);
  await loadAll();
});

document.querySelector("#mapping-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  const data = formData(form);
  data.id = Number(data.id || 0);
  data.provider_id = Number(data.provider_id || 0);
  const method = data.id ? "PUT" : "POST";
  await api("/admin/api/model-mappings", { method, body: JSON.stringify(data) });
  closeForm(form);
  await loadAll();
});

document.querySelector("#key-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  const data = formData(form);
  data.id = Number(data.id || 0);
  data.enabled = form.elements.enabled.checked;
  const method = data.id ? "PUT" : "POST";
  const key = await api("/admin/api/keys", { method, body: JSON.stringify(data) });
  if (key.plain_key) {
    const banner = document.querySelector("#new-key");
    banner.textContent = `${t("new_key")}: ${key.plain_key}`;
    banner.classList.remove("hidden");
  }
  closeForm(form);
  await loadAll();
});

loadAll().catch((error) => {
  alert(error.message);
});
