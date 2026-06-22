const accountState = {
  keys: [],
};

const accountUI = window.TokenFlowUI;
const accountI18n = window.__ACCOUNT_I18N__ || {};
const accountT = (key) => accountI18n[key] || key;
const accountCsrf = () => accountUI.cookie("gateway_account_csrf");

async function accountApi(path, options = {}) {
  return accountUI.api(path, options, { csrf: accountCsrf(), defaultError: accountT("request_failed") });
}

function accountAction(action, value, label, iconName, tone = "secondary") {
  return accountUI.iconActionButton(action, value, label, iconName, tone);
}

function accountCopyButton(value) {
  return accountUI.copyButton(value, accountT("copy"), "copy-account");
}

function accountShowError(error) {
  accountUI.showToast(error?.message || accountT("request_failed"), "error");
}

function renderAccountAddresses() {
  const origin = window.location.origin;
  const target = document.querySelector("#account-api-addresses");
  if (!target) return;
  target.innerHTML = [
    [accountT("api_base"), origin],
    [accountT("openai_chat"), `${origin}/v1/chat/completions`],
    [accountT("openai_models"), `${origin}/v1/models`],
    [accountT("anthropic_messages"), `${origin}/v1/messages`],
    [accountT("anthropic_models"), `${origin}/anthropic/v1/models`],
  ].map(([label, value]) => `
    <div class="api-item">
      <div class="api-item-head"><span>${accountUI.esc(label)}</span>${accountCopyButton(value)}</div>
      <code>${accountUI.esc(value)}</code>
    </div>`).join("");
}

function renderAccountNewKey(plainKey) {
  const banner = document.querySelector("#account-new-key");
  if (!banner) return;
  banner.innerHTML = `<span>${accountUI.esc(accountT("new_key"))}</span><code>${accountUI.esc(plainKey)}</code>${accountCopyButton(plainKey)}`;
  banner.classList.remove("hidden");
}

async function loadAccountKeys(showLoading = true) {
  if (showLoading) accountUI.setRegionLoading("#account-keys", accountT("loading"));
  try {
    accountState.keys = await accountApi("/account/api/keys");
    renderAccountKeys();
  } catch (error) {
    accountUI.setRegionError("#account-keys", error.message || accountT("request_failed"));
    accountShowError(error);
  }
}

function renderAccountKeys() {
  const target = document.querySelector("#account-keys");
  if (!target) return;
  accountUI.clearRegionBusy(target);
  if (!accountState.keys.length) {
    target.innerHTML = `<p class="empty">${accountUI.esc(accountT("empty"))}</p>`;
    return;
  }
  target.innerHTML = `
    <table>
      <thead><tr><th>${accountT("name")}</th><th>${accountT("prefix")}</th><th>${accountT("status")}</th><th>${accountT("requests")}</th><th>${accountT("input_tokens")}</th><th>${accountT("cache_read_tokens")}</th><th>${accountT("output_tokens")}</th><th>${accountT("last_used")}</th><th></th></tr></thead>
      <tbody>${accountState.keys.map((key) => `
        <tr>
          <td>${accountUI.esc(key.name)}</td>
          <td><code>${accountUI.esc(key.prefix)}</code></td>
          <td><span class="status-pill ${key.enabled ? "" : "off"}"><span></span>${key.enabled ? accountT("enabled") : accountT("disabled")}</span></td>
          <td>${accountUI.formatCompactNumber(key.request_count)}</td>
          <td>${accountUI.formatToken(key.input_tokens)}</td>
          <td>${accountUI.formatToken(key.cache_read_tokens)}</td>
          <td>${accountUI.formatToken(key.output_tokens)}</td>
          <td>${accountUI.date(key.last_used_at)}</td>
          <td class="actions">
            ${accountAction("edit-account-key", key.id, accountT("edit"), "edit")}
            ${accountAction("reset-account-key", key.id, accountT("regenerate"), "refresh")}
            ${accountAction("delete-account-key", key.id, accountT("delete"), "trash", "danger")}
          </td>
        </tr>`).join("")}</tbody>
    </table>`;
}

function openAccountKeyForm(key) {
  const form = document.querySelector("#account-key-form");
  if (!form) return;
  form.reset();
  form.elements.id.value = key?.id || "";
  form.elements.name.value = key?.name || "";
  form.elements.enabled.checked = key ? !!key.enabled : true;
  form.querySelector(".account-key-enabled")?.classList.toggle("hidden", !key);
  form.classList.remove("hidden");
  document.querySelector("#account-new-key")?.classList.add("hidden");
}

function closeAccountKeyForm() {
  const form = document.querySelector("#account-key-form");
  if (!form) return;
  form.reset();
  form.classList.add("hidden");
}

async function accountConfirmDanger(message, confirmLabel) {
  return accountUI.confirmAction({
    title: accountT("confirm_title"),
    message,
    confirmLabel,
    cancelLabel: accountT("cancel"),
    tone: "danger",
  });
}

async function handleAccountClick(event) {
  const eventTarget = event.target;
  if (!(eventTarget instanceof Element)) return;
  const target = eventTarget.closest("button") || eventTarget;
  if (target.matches("[data-copy-account]")) {
    await accountUI.copyText(target.dataset.copyAccount || "");
    accountUI.showToast(accountT("copied"));
    return;
  }
  if (target.matches("[data-open-account-key]")) {
    openAccountKeyForm(null);
    return;
  }
  if (target.matches("[data-cancel-account-key]")) {
    closeAccountKeyForm();
    return;
  }
  if (target.matches("[data-edit-account-key]")) {
    const key = accountState.keys.find((item) => String(item.id) === target.dataset.editAccountKey);
    if (key) openAccountKeyForm(key);
    return;
  }
  if (target.matches("[data-reset-account-key]")) {
    if (await accountConfirmDanger(accountT("reset_key_confirm"), accountT("regenerate"))) {
      const key = await accountApi("/account/api/keys/reset", { method: "POST", body: JSON.stringify({ id: Number(target.dataset.resetAccountKey || 0) }) });
      await loadAccountKeys();
      if (key.plain_key) renderAccountNewKey(key.plain_key);
      accountUI.showToast(accountT("saved"));
    }
    return;
  }
  if (target.matches("[data-delete-account-key]")) {
    if (await accountConfirmDanger(accountT("delete_key_confirm"), accountT("delete"))) {
      await accountApi(`/account/api/keys?id=${target.dataset.deleteAccountKey}`, { method: "DELETE" });
      accountUI.showToast(accountT("saved"));
      await loadAccountKeys();
    }
  }
}

document.addEventListener("click", (event) => {
  handleAccountClick(event).catch(accountShowError);
});

document.querySelector("#account-key-form")?.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  try {
    await accountUI.withFormBusy(form, async () => {
      const data = Object.fromEntries(new FormData(form).entries());
      data.id = Number(data.id || 0);
      data.enabled = form.elements.enabled.checked;
      const method = data.id ? "PUT" : "POST";
      const key = await accountApi("/account/api/keys", { method, body: JSON.stringify(data) });
      closeAccountKeyForm();
      await loadAccountKeys();
      if (key.plain_key) renderAccountNewKey(key.plain_key);
    }, accountT("saving"));
    accountUI.showToast(accountT("saved"));
  } catch (error) {
    accountShowError(error);
  }
});

renderAccountAddresses();
loadAccountKeys().catch(accountShowError);
