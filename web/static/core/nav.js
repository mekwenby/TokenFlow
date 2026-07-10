const defaultView = "overview";

const validViews = new Set([
  "overview",
  "api-addresses",
  "users",
  "providers",
  "mappings",
  "keys",
  "logs",
  "more",
]);

const legacyViews = new Map([
  ["api-addresses-section", "api-addresses"],
  ["users-section", "users"],
  ["providers-section", "providers"],
  ["mappings-section", "mappings"],
  ["keys-section", "keys"],
  ["logs-section", "logs"],
]);

export function normalizeAdminView(hash = "") {
  let value = String(hash || "").replace(/^#/, "");
  try {
    value = decodeURIComponent(value);
  } catch {
    return defaultView;
  }
  value = legacyViews.get(value) || value;
  return validViews.has(value) ? value : defaultView;
}

function canonicalHash(view) {
  return `#${view}`;
}

function activeViews(link) {
  const values = link.dataset.navActiveFor || link.dataset.navView || "";
  return values.split(/\s+/).filter(Boolean);
}

// Retained for the account portal, which still uses scroll-based section navigation.
export function initSectionNav() {
  const links = Array.from(document.querySelectorAll("[data-nav-link]"));
  if (!links.length) return;

  const byId = new Map();
  for (const link of links) {
    const href = link.getAttribute("href") || "";
    if (!href.startsWith("#") || href.length < 2) continue;
    const id = decodeURIComponent(href.slice(1));
    if (!byId.has(id)) byId.set(id, []);
    byId.get(id).push(link);
  }

  const setActive = (id) => {
    for (const link of links) {
      const active = (link.getAttribute("href") || "") === `#${id}`;
      link.classList.toggle("active", active);
      if (active) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    }
  };

  const sections = Array.from(byId.keys())
    .map((id) => document.getElementById(id))
    .filter(Boolean);
  if (!sections.length) return;

  if (!("IntersectionObserver" in window)) {
    setActive(sections[0].id);
    return;
  }

  const observer = new IntersectionObserver((entries) => {
    const visible = entries
      .filter((entry) => entry.isIntersecting)
      .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0];
    if (visible?.target?.id) setActive(visible.target.id);
  }, { rootMargin: "-18% 0px -68% 0px", threshold: [0.1, 0.35, 0.6] });

  sections.forEach((section) => observer.observe(section));
  const initial = location.hash ? normalizeLegacySectionHash(location.hash) : sections[0].id;
  setActive(sections.some((section) => section.id === initial) ? initial : sections[0].id);
}

function normalizeLegacySectionHash(hash) {
  try {
    return decodeURIComponent(String(hash || "").replace(/^#/, ""));
  } catch {
    return "";
  }
}

export function initViewNav({ onViewChange } = {}) {
  const links = Array.from(document.querySelectorAll("[data-nav-view]"));
  const views = Array.from(document.querySelectorAll("[data-admin-view]"));
  if (!views.length) return null;

  const baseTitle = document.title;
  let currentView = "";

  const replaceHash = (view) => {
    const url = `${location.pathname}${location.search}${canonicalHash(view)}`;
    history.replaceState(history.state, "", url);
  };

  const activate = (view, { focus = false } = {}) => {
    const previousView = currentView;
    currentView = view;

    for (const section of views) {
      section.hidden = section.dataset.adminView !== view;
    }
    for (const link of links) {
      const active = activeViews(link).includes(view);
      link.classList.toggle("active", active);
      if (active) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    }

    const section = views.find((item) => item.dataset.adminView === view);
    const heading = section?.querySelector("h2");
    const title = heading?.textContent?.trim();
    document.title = title ? `${title} - ${baseTitle}` : baseTitle;

    if (focus && previousView && previousView !== view && heading) {
      heading.setAttribute("tabindex", "-1");
      heading.focus({ preventScroll: true });
    }

    Promise.resolve(onViewChange?.(view, previousView)).catch(() => {});
  };

  const syncFromHash = ({ focus = false } = {}) => {
    const view = normalizeAdminView(location.hash);
    if (location.hash !== canonicalHash(view)) replaceHash(view);
    activate(view, { focus });
  };

  window.addEventListener("hashchange", () => syncFromHash({ focus: true }));
  syncFromHash();

  return {
    get currentView() { return currentView; },
    activate: (view) => {
      const normalized = normalizeAdminView(view);
      if (location.hash === canonicalHash(normalized)) activate(normalized, { focus: true });
      else location.hash = normalized;
    },
  };
}
