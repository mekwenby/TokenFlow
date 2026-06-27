import { showToast } from "../core/toast.js";
import { confirmAction } from "../core/confirm.js";
import { withFormBusy } from "../core/dom.js";

/**
 * Opens a form section by id.
 */
export function openForm(formId) {
  const form = document.getElementById(formId);
  if (!form) return;
  form.classList.remove("hidden");
  form.elements[0]?.focus();
}

/**
 * Closes a form, resetting it and hiding it.
 */
export function closeForm(form) {
  if (!form) return;
  form.classList.add("hidden");
  form.reset();
  // Clear any hidden id field
  const idField = form.elements["id"];
  if (idField) idField.value = "";
}

/**
 * Populates form fields from a data object.
 */
export function populateForm(form, data, fields) {
  for (const key of fields) {
    const el = form.elements[key];
    if (!el) continue;
    if (el.type === "checkbox") {
      el.checked = !!data[key];
    } else {
      el.value = data[key] ?? "";
    }
  }
}

/**
 * Handles form submission with busy state, toast, and callback.
 */
export async function submitForm(form, task, onSuccess) {
  await withFormBusy(form, async () => {
    try {
      const result = await task();
      showToast("Saved");
      closeForm(form);
      if (onSuccess) await onSuccess(result);
    } catch (error) {
      showToast(error?.message || "Request failed", "error");
      throw error;
    }
  });
}

/**
 * Confirms and deletes an entity, then runs onSuccess.
 */
export async function confirmDelete({ message, confirmLabel, apiCall, onSuccess, t }) {
  if (!(await confirmAction({
    title: t("confirm_title"),
    message,
    confirmLabel: confirmLabel || t("continue"),
    cancelLabel: t("cancel"),
    tone: "danger",
  }))) return;
  await apiCall();
  showToast(t("saved"));
  if (onSuccess) await onSuccess();
}
