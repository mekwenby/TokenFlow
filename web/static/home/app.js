export function shouldRunMotion({ reducedMotion, intersecting, pageVisible }) {
  return !reducedMotion && intersecting && pageVisible;
}

export function homeMotionState(options) {
  if (options.reducedMotion) return "reduced";
  return shouldRunMotion(options) ? "running" : "paused";
}

export function initHomeMotion() {
  const page = document.querySelector(".home-page");
  const targets = Array.from(document.querySelectorAll("[data-home-motion]"));
  if (!page || !targets.length) return () => {};

  const motionQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
  const intersections = new WeakMap(targets.map((target) => [target, false]));
  let pageVisible = document.visibilityState === "visible";

  const sync = () => {
    const reducedMotion = motionQuery.matches;
    page.dataset.motionPreference = reducedMotion ? "reduced" : "full";
    targets.forEach((target) => {
      const intersecting = intersections.get(target) === true;
      target.dataset.motionState = homeMotionState({ reducedMotion, intersecting, pageVisible });
      if (intersecting) target.classList.add("motion-revealed");
    });
  };

  const onVisibilityChange = () => {
    pageVisible = document.visibilityState === "visible";
    sync();
  };
  const onMotionPreferenceChange = () => sync();

  let observer;
  if ("IntersectionObserver" in window) {
    observer = new IntersectionObserver((entries) => {
      entries.forEach((entry) => intersections.set(entry.target, Boolean(entry.isIntersecting && entry.intersectionRatio >= 0.12)));
      sync();
    }, { threshold: [0, 0.12, 0.5] });
    targets.forEach((target) => observer.observe(target));
  } else {
    targets.forEach((target) => intersections.set(target, true));
  }

  document.addEventListener("visibilitychange", onVisibilityChange);
  if (motionQuery.addEventListener) motionQuery.addEventListener("change", onMotionPreferenceChange);
  else motionQuery.addListener(onMotionPreferenceChange);
  sync();

  return () => {
    observer?.disconnect();
    document.removeEventListener("visibilitychange", onVisibilityChange);
    if (motionQuery.removeEventListener) motionQuery.removeEventListener("change", onMotionPreferenceChange);
    else motionQuery.removeListener(onMotionPreferenceChange);
  };
}

if (typeof window !== "undefined" && typeof document !== "undefined") {
  initHomeMotion();
}
