let resolver;
let previousFocus;

function ensureDialog() {
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
    if (event.target === modal) close(false);
  });
  modal.addEventListener("keydown", (event) => {
    if (modal.classList.contains("hidden")) return;
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      close(false);
      return;
    }
    if (event.key !== "Tab") return;
    event.stopPropagation();
    const focusable = Array.from(modal.querySelectorAll("button")).filter((b) => !b.disabled);
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
  modal.querySelector("[data-confirm-cancel]").addEventListener("click", () => close(false));
  modal.querySelector("[data-confirm-ok]").addEventListener("click", () => close(true));
  return modal;
}

function close(result) {
  const modal = document.querySelector("#confirm-modal");
  if (!modal || modal.classList.contains("hidden")) return;
  modal.classList.add("hidden");
  modal.setAttribute("aria-hidden", "true");
  if (resolver) resolver(result);
  resolver = undefined;
  if (previousFocus && typeof previousFocus.focus === "function") {
    previousFocus.focus();
  }
  previousFocus = undefined;
}

export function confirmAction(options = {}) {
  const modal = ensureDialog();
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
    resolver = resolve;
  });
}
