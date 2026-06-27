export function createAPIClient({ csrfCookie, defaultError = "Request failed" }) {
  function csrf() {
    return document.cookie
      .split("; ")
      .find((row) => row.startsWith(`${csrfCookie}=`))
      ?.split("=")[1] || "";
  }

  return async function api(path, options = {}) {
    const headers = { "Content-Type": "application/json", ...(options.headers || {}) };
    if (options.method && options.method !== "GET" && csrfCookie) {
      headers["X-CSRF-Token"] = csrf();
    }
    const response = await fetch(path, { ...options, headers });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error(body.error || defaultError || "Request failed");
    }
    return body;
  };
}
