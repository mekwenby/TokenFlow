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
      link.setAttribute("aria-current", active ? "page" : "false");
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
  setActive(location.hash ? decodeURIComponent(location.hash.slice(1)) : sections[0].id);
}
