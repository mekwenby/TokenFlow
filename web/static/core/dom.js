export function esc(value) {
  return String(value ?? "").replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  })[ch]);
}

export function cookie(name) {
  return document.cookie
    .split("; ")
    .find((row) => row.startsWith(`${name}=`))
    ?.split("=")[1] || "";
}

const resolveTarget = (target) =>
  typeof target === "string" ? document.querySelector(target) : target;

export function loadingHTML(message = "Loading...") {
  return `<p class="empty loading">${esc(message)}</p>`;
}

export function errorHTML(message = "Request failed") {
  return `<p class="inline-error" role="alert">${esc(message)}</p>`;
}

export function setRegionLoading(target, message) {
  const el = resolveTarget(target);
  if (!el) return;
  el.setAttribute("aria-busy", "true");
  el.innerHTML = loadingHTML(message);
}

export function setRegionError(target, message) {
  const el = resolveTarget(target);
  if (!el) return;
  el.removeAttribute("aria-busy");
  el.innerHTML = errorHTML(message);
}

export function clearRegionBusy(target) {
  const el = resolveTarget(target);
  if (el) el.removeAttribute("aria-busy");
}

export async function withFormBusy(form, task, busyLabel) {
  const buttons = Array.from(form.querySelectorAll('button[type="submit"]'));
  const originals = buttons.map((btn) => ({
    button: btn,
    text: btn.textContent,
    disabled: btn.disabled,
  }));
  form.setAttribute("aria-busy", "true");
  buttons.forEach((btn) => {
    btn.disabled = true;
    btn.classList.add("busy");
    if (busyLabel) btn.textContent = busyLabel;
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
