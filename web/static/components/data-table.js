import { esc } from "../core/dom.js";
import { formatToken, date } from "../core/format.js";
import { iconActionButton } from "../core/toast.js";

/**
 * Renders a data table into a container element.
 *
 * @param {Element} container
 * @param {Array<{label: string, key?: string, render?: (row: any) => string}>} columns
 * @param {Array} rows
 * @param {string} emptyText
 * @param {(row: any) => string} [actions] — optional action buttons HTML
 */
export function renderTable(container, { columns, rows, emptyText, actions }) {
  if (!rows.length) {
    container.innerHTML = `<p class="empty">${esc(emptyText)}</p>`;
    return;
  }
  const header = columns.map((col) => `<th>${esc(col.label)}</th>`).join("");
  const body = rows.map((row) => {
    const cells = columns.map((col) => {
      if (col.render) return `<td>${col.render(row)}</td>`;
      return `<td>${esc(row[col.key] ?? "")}</td>`;
    }).join("");
    const actionHTML = actions ? `<td class="actions">${actions(row)}</td>` : "";
    return `<tr>${cells}${actionHTML}</tr>`;
  }).join("");
  container.innerHTML = `<table><thead><tr>${header}</tr></thead><tbody>${body}</tbody></table>`;
}

/** Helper to render a status pill. */
export function statusPill(on, enabledLabel, disabledLabel) {
  const label = on ? enabledLabel : disabledLabel;
  return `<span class="status-pill ${on ? "" : "off"}" title="${esc(label)}" aria-label="${esc(label)}"><span></span>${esc(label)}</span>`;
}
