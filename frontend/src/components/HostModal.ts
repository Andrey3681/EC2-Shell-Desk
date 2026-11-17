import { t } from "../i18n";
import {
  desktopApi,
  type HostGroup,
  type HostInstance,
  type SSHKeySummary,
} from "../services/desktopApi";
import { escapeHtml, extractErrorMessage } from "./dom";
import { openGroupModal } from "./GroupModal";

export function openHostModal(
  existing: HostInstance | null,
  keys: SSHKeySummary[],
  groups: HostGroup[],
  onSaved: () => void,
  defaultGroupId = ""
): void {
  const copy = t();
  const backdrop = document.createElement("div");
  backdrop.className = "modal-backdrop";
  let errorMsg = "";

  let currentGroups = [...groups];
  let currentGroupId = existing?.groupId ?? defaultGroupId;
  let currentAlias = existing?.alias ?? "";
  let currentHostAddr = existing?.hostAddress ?? "";
  let currentPort = existing?.port ?? 22;
  let currentSSHUser = existing?.sshUsername ?? "ec2-user";
  let currentEnv = existing?.environment ?? "Prod";
  let currentKeyId = existing?.sshKeyId ?? "";
  let currentNotes = existing?.notes ?? "";

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

  const syncInputs = () => {
    currentGroupId = backdrop.querySelector<HTMLSelectElement>("#host-group")?.value ?? "";
    currentAlias = backdrop.querySelector<HTMLInputElement>("#host-alias")?.value ?? "";
    currentHostAddr = backdrop.querySelector<HTMLInputElement>("#host-addr")?.value ?? "";
    currentPort = Number(backdrop.querySelector<HTMLInputElement>("#host-port")?.value ?? "22");
    currentSSHUser = backdrop.querySelector<HTMLInputElement>("#host-user")?.value ?? "";
    currentEnv = backdrop.querySelector<HTMLSelectElement>("#host-env")?.value ?? "Prod";
    currentKeyId = backdrop.querySelector<HTMLSelectElement>("#host-key")?.value ?? "";
    currentNotes = backdrop.querySelector<HTMLTextAreaElement>("#host-notes")?.value ?? "";
  };

  const render = () => {
    backdrop.innerHTML = `
      <div class="modal-card">
        <div class="modal-header">
          <h2 class="modal-title">${escapeHtml(existing ? copy.hostModal.editTitle : copy.hostModal.createTitle)}</h2>
          <button id="host-close" class="btn btn-sm">${escapeHtml(copy.common.close)}</button>
        </div>
        ${errorMsg ? `<div class="error-banner">${escapeHtml(errorMsg)}</div>` : ""}
        <form id="host-form">
          <div class="field">
            <label for="host-group">${escapeHtml(copy.hostModal.group)}</label>
            <div style="display:flex;gap:8px;">
              <select id="host-group" class="select" style="flex:1;">
                <option value="">${escapeHtml(copy.hostModal.noGroup)}</option>
                ${currentGroups
                  .map(
                    (g) =>
                      `<option value="${escapeHtml(g.id)}" ${currentGroupId === g.id ? "selected" : ""}>${escapeHtml(g.name)}</option>`
                  )
                  .join("")}
              </select>
              <button type="button" id="host-quick-group" class="btn btn-sm">${escapeHtml(copy.hostModal.quickNewGroup)}</button>
            </div>
          </div>
          <div class="field">
            <label for="host-alias">${escapeHtml(copy.hostModal.alias)}</label>
            <input id="host-alias" class="input" required value="${escapeHtml(currentAlias)}" placeholder="${escapeHtml(copy.hostModal.aliasPlaceholder)}" />
          </div>
          <div class="row-2col">
            <div class="field">
              <label for="host-addr">${escapeHtml(copy.hostModal.hostAddress)}</label>
              <input id="host-addr" class="input mono" required value="${escapeHtml(currentHostAddr)}" placeholder="${escapeHtml(copy.hostModal.hostPlaceholder)}" />
            </div>
            <div class="field">
              <label for="host-port">${escapeHtml(copy.hostModal.port)}</label>
              <input id="host-port" class="input mono" type="number" min="1" max="65535" required value="${currentPort}" />
            </div>
          </div>
          <div class="row-2col">
            <div class="field">
              <label for="host-user">${escapeHtml(copy.hostModal.sshUsername)}</label>
              <input id="host-user" class="input mono" required value="${escapeHtml(currentSSHUser)}" placeholder="${escapeHtml(copy.hostModal.sshUserPlaceholder)}" />
            </div>
            <div class="field">
              <label for="host-env">${escapeHtml(copy.hostModal.environment)}</label>
              <select id="host-env" class="select">
                ${["Prod", "Staging", "Dev"]
                  .map(
                    (env) =>
                      `<option value="${env}" ${currentEnv === env ? "selected" : ""}>${env}</option>`
                  )
                  .join("")}
              </select>
            </div>
          </div>
          <div class="field">
            <label for="host-key">${escapeHtml(copy.hostModal.sshKey)}</label>
            <select id="host-key" class="select">
              <option value="">${escapeHtml(copy.hostModal.selectKeyNone)}</option>
              ${keys
                .map(
                  (k) =>
                    `<option value="${escapeHtml(k.id)}" ${currentKeyId === k.id ? "selected" : ""}>${escapeHtml(k.name)} (${escapeHtml(k.keyType)})</option>`
                )
                .join("")}
            </select>
          </div>
          <div class="field">
            <label for="host-notes">${escapeHtml(copy.hostModal.notes)}</label>
            <textarea id="host-notes" class="textarea" rows="2" placeholder="${escapeHtml(copy.hostModal.notesPlaceholder)}">${escapeHtml(currentNotes)}</textarea>
          </div>
          <div class="modal-footer">
            <button type="button" id="host-cancel" class="btn">${escapeHtml(copy.common.cancel)}</button>
            <button type="submit" class="btn btn-primary">${escapeHtml(copy.common.save)}</button>
          </div>
        </form>
      </div>
    `;

    backdrop.querySelector("#host-close")?.addEventListener("click", closeModal);
    backdrop.querySelector("#host-cancel")?.addEventListener("click", closeModal);

    backdrop.querySelector("#host-quick-group")?.addEventListener("click", () => {
      syncInputs();
      openGroupModal(null, (created) => {
        currentGroups = [...currentGroups, created].sort((a, b) =>
          a.name.localeCompare(b.name)
        );
        currentGroupId = created.id;
        render();
      });
    });

    backdrop.querySelector<HTMLFormElement>("#host-form")?.addEventListener("submit", async (e) => {
      e.preventDefault();
      syncInputs();
      errorMsg = "";
      const payload = {
        id: existing?.id ?? "",
        groupId: currentGroupId.trim(),
        alias: currentAlias.trim(),
        hostAddress: currentHostAddr.trim(),
        port: currentPort,
        sshUsername: currentSSHUser.trim(),
        sshKeyId: currentKeyId,
        environment: currentEnv,
        notes: currentNotes.trim(),
      };

      try {
        if (existing) {
          await desktopApi.hosts.update(payload);
        } else {
          await desktopApi.hosts.create(payload);
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
