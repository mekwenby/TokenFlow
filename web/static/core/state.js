/** Simple reactive store backed by EventTarget. */
export class Store {
  #state;
  #emitter = new EventTarget();

  constructor(initial = {}) {
    this.#state = { ...initial };
  }

  get(key) {
    return this.#state[key];
  }

  getAll() {
    return { ...this.#state };
  }

  set(key, value) {
    this.#state[key] = value;
    this.#emitter.dispatchEvent(new CustomEvent("change", { detail: { key, value } }));
  }

  update(obj) {
    Object.assign(this.#state, obj);
    this.#emitter.dispatchEvent(new CustomEvent("change", { detail: obj }));
  }

  onChange(fn) {
    const handler = (e) => fn(e.detail);
    this.#emitter.addEventListener("change", handler);
    return () => this.#emitter.removeEventListener("change", handler);
  }
}
