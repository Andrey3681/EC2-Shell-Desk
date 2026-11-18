import { t } from "../i18n";
import {
  desktopApi,
  type CommandSnippet,
  type HostGroup,
  type HostInstance,
  type SaveSnippetInput,
} from "../services/desktopApi";
import { escapeHtml, extractErrorMessage } from "./dom";

export function openSnippetModal(
  existing: CommandSnippet | null,
  hosts: HostInstance[],
  groups: HostGroup[],
  onSaved: () => void,
  defaultHostId = ""
): void {
  const copy = t();
  const backdrop = document.createElement("div");
  backdrop.className = "modal-backdrop";

  let errorMsg = "";
  let name = existing?.name ?? "";
  let hostId = existing ? existing.hostId : defaultHostId;
  let groupId = existing?.groupId ?? "";
  let command = existing?.command ?? "";
  let autoExecute = existing ? existing.autoExecute : true;
  let notes = existing?.notes ?? "";

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
      <div class="modal-card" style="max-width:540px;">
        <div class="modal-header">
          <h2 class="modal-title">${escapeHtml(existing ? copy.snippetModal.editTitle : copy.snippetModal.createTitle)}</h2>
          <button id="snip-close" class="btn btn-sm">${escapeHtml(copy.common.close)}</button>
        </div>
        ${errorMsg ? `<div class="error-banner">${escapeHtml(errorMsg)}</div>` : ""}
        <form id="snip-form">
          <div class="field">
            <label for="snip-name">${escapeHtml(copy.snippetModal.name)}</label>
            <input id="snip-name" class="input" required maxlength="128" value="${escapeHtml(name)}" placeholder="${escapeHtml(copy.snippetModal.namePlaceholder)}" />
          </div>
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:10px;">
            <div class="field">
              <label for="snip-host">${escapeHtml(copy.snippetModal.host)}</label>
              <select id="snip-host" class="select">
                <option value="" ${hostId === "" ? "selected" : ""}>${escapeHtml(copy.snippetModal.noHost)}</option>
                ${hosts
                  .map(
                    (h) =>
                      `<option value="${escapeHtml(h.id)}" ${hostId === h.id ? "selected" : ""}>${escapeHtml(h.alias)} (${escapeHtml(h.sshUsername)}@${escapeHtml(h.hostAddress)})</option>`
                  )
                  .join("")}
              </select>
            </div>
            <div class="field">
              <label for="snip-group">${escapeHtml(copy.snippetModal.group)}</label>
              <select id="snip-group" class="select">
                <option value="" ${groupId === "" ? "selected" : ""}>${escapeHtml(copy.snippetModal.noGroup)}</option>
                ${groups
                  .map(
                    (g) =>
                      `<option value="${escapeHtml(g.id)}" ${groupId === g.id ? "selected" : ""}>${escapeHtml(g.name)}</option>`
                  )
                  .join("")}
              </select>
            </div>
          </div>
          <div class="field">
            <label for="snip-command">${escapeHtml(copy.snippetModal.command)}</label>
            <textarea id="snip-command" class="input mono" rows="6" required maxlength="16384" placeholder="${escapeHtml(copy.snippetModal.commandPlaceholder)}">${escapeHtml(command)}</textarea>
          </div>
          <div class="field">
            <label style="display:flex;align-items:center;gap:8px;cursor:pointer;font-weight:500;">
              <input id="snip-auto-exec" type="checkbox" ${autoExecute ? "checked" : ""} />
              <span>${escapeHtml(copy.snippetModal.autoExecute)}</span>
            </label>
          </div>
          <div class="field">
            <label for="snip-notes">${escapeHtml(copy.snippetModal.notes)}</label>
            <input id="snip-notes" class="input" maxlength="512" value="${escapeHtml(notes)}" placeholder="${escapeHtml(copy.snippetModal.notesPlaceholder)}" />
          </div>
          <div class="modal-footer">
            <button type="button" id="snip-cancel" class="btn">${escapeHtml(copy.common.cancel)}</button>
            <button type="submit" class="btn btn-primary">${escapeHtml(copy.common.save)}</button>
          </div>
        </form>
      </div>
    `;

    backdrop.querySelector("#snip-close")?.addEventListener("click", closeModal);
    backdrop.querySelector("#snip-cancel")?.addEventListener("click", closeModal);

    const nameEl = backdrop.querySelector<HTMLInputElement>("#snip-name");
    const hostEl = backdrop.querySelector<HTMLSelectElement>("#snip-host");
    const groupEl = backdrop.querySelector<HTMLSelectElement>("#snip-group");
    const cmdEl = backdrop.querySelector<HTMLTextAreaElement>("#snip-command");
    const autoEl = backdrop.querySelector<HTMLInputElement>("#snip-auto-exec");
    const notesEl = backdrop.querySelector<HTMLInputElement>("#snip-notes");
    nameEl?.focus();

    backdrop.querySelector<HTMLFormElement>("#snip-form")?.addEventListener("submit", async (e) => {
      e.preventDefault();
      errorMsg = "";
      name = nameEl?.value.trim() ?? "";
      hostId = hostEl?.value ?? "";
      groupId = groupEl?.value ?? "";
      command = cmdEl?.value ?? "";
      autoExecute = autoEl?.checked ?? true;
      notes = notesEl?.value.trim() ?? "";

      const payload: SaveSnippetInput = {
        hostId,
        groupId,
        name,
        command,
        autoExecute,
        notes,
      };
      if (existing) {
        payload.id = existing.id;
      }

      try {
        if (existing) {
          await desktopApi.snippets.update(payload);
        } else {
          await desktopApi.snippets.create(payload);
        }
        closeModal();
        onSaved();
      } catch (err) {
        errorMsg = extractErrorMessage(err);
        render();
      }
    });
  };

  render();
  document.body.appendChild(backdrop);
}
