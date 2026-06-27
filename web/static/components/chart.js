import { esc } from "../core/dom.js";
import { formatToken } from "../core/format.js";

/**
 * Renders a horizontal bar chart as inline SVG.
 *
 * @param {{ items: Array<{label: string, values: Record<string, number>}>, series: Array<{key: string, color: string, label: string}>, maxLabelChars?: number }} config
 * @returns {string} HTML string
 */
export function renderBarChart({ items, series, maxLabelChars = 26 }) {
  if (!items.length) return "";
  const width = 900;
  const left = 190;
  const right = 82;
  const top = 28;
  const rowHeight = 70;
  const barH = 8;
  const gap = 12;
  const height = top + items.length * rowHeight + 16;
  const plotW = width - left - right;

  const maxVal = Math.max(...items.flatMap((item) => series.map((s) => item.values[s.key] || 0)));
  const niceMax = maxVal > 0 ? Math.ceil(maxVal / 10 ** Math.floor(Math.log10(maxVal) || 1)) * 10 ** Math.floor(Math.log10(maxVal) || 1) : 1;

  const trunc = (v) => String(v || "-").length > maxLabelChars ? `${String(v || "-").slice(0, maxLabelChars - 1)}...` : String(v || "-");

  const legend = series.map((s) => `
    <span class="usage-legend-item">
      <span class="usage-color" style="background:${s.color}"></span>
      <span>${esc(s.label)}</span>
    </span>`).join("");

  const rows = items.map((item, i) => {
    const y = top + i * rowHeight;
    const label = trunc(item.label);
    const bars = series.map((s, si) => {
      const val = item.values[s.key] || 0;
      const by = y + 8 + si * gap;
      const bw = val > 0 ? Math.max(2, (val / niceMax) * plotW) : 0;
      return `
        <rect class="detail-chart-bar-bg" x="${left}" y="${by}" width="${plotW}" height="${barH}" rx="4"></rect>
        <rect class="detail-chart-bar" x="${left}" y="${by}" width="${bw.toFixed(2)}" height="${barH}" rx="4" fill="${s.color}"></rect>
        <text class="chart-axis" x="${left + bw + 8}" y="${by + 8}">${formatToken(val)}</text>
        <title>${esc(`${item.label || "-"} ${s.label}: ${formatToken(val)}`)}</title>`;
    }).join("");
    return `<g><text class="detail-chart-label" x="${left - 12}" y="${y + 32}" text-anchor="end">${esc(label)}</text>${bars}</g>`;
  }).join("");

  return `
    <div class="detail-chart-panel">
      <div class="usage-summary"><span></span><div class="usage-legend">${legend}</div></div>
      <div class="detail-chart-scroll">
        <svg class="detail-chart-svg" viewBox="0 0 ${width} ${height}" role="img"><title></title>${rows}</svg>
      </div>
    </div>`;
}

/**
 * Renders a line chart for token usage over time.
 */
export function renderLineChart({ points, series, width = 900, height = 300 }) {
  if (!points.length) return "";
  const margin = { top: 24, right: 32, bottom: 48, left: 72 };
  const plotW = width - margin.left - margin.right;
  const plotH = height - margin.top - margin.bottom;

  const maxVal = Math.max(...points.flatMap((p) => series.map((s) => p[s.key] || 0)));
  const niceMax = maxVal > 0 ? Math.ceil(maxVal / 10 ** Math.floor(Math.log10(maxVal) || 1)) * 10 ** Math.floor(Math.log10(maxVal) || 1) : 1;

  // Y-axis ticks
  const yTicks = 5;
  const yLines = Array.from({ length: yTicks }, (_, i) => {
    const val = (niceMax / (yTicks - 1)) * i;
    const y = margin.top + plotH - (val / niceMax) * plotH;
    return `<g>
      <line class="chart-grid" x1="${margin.left}" y1="${y.toFixed(1)}" x2="${width - margin.right}" y2="${y.toFixed(1)}"></line>
      <text class="chart-axis" x="${margin.left - 8}" y="${y + 4}" text-anchor="end">${formatToken(val)}</text>
    </g>`;
  }).join("");

  // X-axis labels
  const xLabels = points.filter((_, i) => i % Math.max(1, Math.floor(points.length / 8)) === 0 || i === points.length - 1)
    .map((p) => {
      const i = points.indexOf(p);
      const x = margin.left + (i / Math.max(1, points.length - 1)) * plotW;
      return `<text class="chart-axis" x="${x.toFixed(1)}" y="${height - 8}" text-anchor="middle">${esc(p.label || "")}</text>`;
    }).join("");

  // Series lines
  const paths = series.map((s) => {
    const d = points.map((p, i) => {
      const x = margin.left + (i / Math.max(1, points.length - 1)) * plotW;
      const y = margin.top + plotH - ((p[s.key] || 0) / niceMax) * plotH;
      return `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`;
    }).join(" ");
    return `<path class="chart-line" d="${d}" stroke="${s.color}" fill="none"></path>`;
  }).join("");

  // Legend
  const legend = series.map((s) => `
    <span class="usage-legend-item">
      <span class="usage-color" style="background:${s.color}"></span>
      <span>${esc(s.label)}</span>
    </span>`).join("");

  return `
    <div class="usage-summary"><div class="usage-legend">${legend}</div></div>
    <svg class="chart-svg" viewBox="0 0 ${width} ${height}" role="img">
      ${yLines}
      ${paths}
      ${xLabels}
    </svg>`;
}
