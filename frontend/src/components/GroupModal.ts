import { t } from "../i18n";
import { desktopApi, type HostGroup } from "../services/desktopApi";
import { escapeHtml, extractErrorMessage } from "./dom";

export function openGroupModal(
  existing: HostGroup | null,
  onSaved: (group: HostGroup) => void
): void {
  const copy = t();
  const backdrop = document.createElement("div");
  backdrop.className = "modal-backdrop";
  let errorMsg = "";

  const closeModal = () => {
    window.removeEventListener("keydown", onKeyDown);
    backdrop.remove();
  };
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Escape") closeModal();
  };
  window.addEventListener("keydown", onKeyDown);
  backdrop.addEventListener("click", (e) => {
    if (e.target === backdrop) closeModal();
  });

  const render = () => {
    backdrop.innerHTML = `
      <div class="modal-card" style="max-width:420px;">
        <div class="modal-header">
          <h2 class="modal-title">${escapeHtml(existing ? copy.sidebar.renameGroupTitle : copy.sidebar.groupNameTitle)}</h2>
          <button id="grp-close" class="btn btn-sm">${escapeHtml(copy.common.close)}</button>
        </div>
        ${errorMsg ? `<div class="error-banner">${escapeHtml(errorMsg)}</div>` : ""}
        <form id="grp-form">
          <div class="field">
            <label for="grp-name">${escapeHtml(copy.sidebar.groupNameLabel)}</label>
            <input id="grp-name" class="input" required maxlength="64" value="${escapeHtml(existing?.name ?? "")}" placeholder="${escapeHtml(copy.sidebar.groupNamePlaceholder)}" />
          </div>
          <div class="modal-footer">
            <button type="button" id="grp-cancel" class="btn">${escapeHtml(copy.common.cancel)}</button>
            <button type="submit" class="btn btn-primary">${escapeHtml(copy.common.save)}</button>
          </div>
        </form>
      </div>
    `;

    backdrop.querySelector("#grp-close")?.addEventListener("click", closeModal);
    backdrop.querySelector("#grp-cancel")?.addEventListener("click", closeModal);

    const inputEl = backdrop.querySelector<HTMLInputElement>("#grp-name");
    inputEl?.focus();

    backdrop.querySelector<HTMLFormElement>("#grp-form")?.addEventListener("submit", async (e) => {
      e.preventDefault();
      errorMsg = "";
      const name = inputEl?.value.trim() ?? "";
      try {
        const saved = existing
          ? await desktopApi.groups.rename(existing.id, name)
          : await desktopApi.groups.create(name);
        closeModal();
        onSaved(saved);
      } catch (err) {
        errorMsg = extractErrorMessage(err);
        render();
      }
    });
  };

  render();
  document.body.appendChild(backdrop);
}
