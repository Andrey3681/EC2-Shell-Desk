import { t } from "../i18n";
import { desktopApi, type HostInstance, type SSHKeySummary } from "../services/desktopApi";
import { escapeHtml, extractErrorMessage } from "./dom";

export function openHostModal(
  existing: HostInstance | null,
  keys: SSHKeySummary[],
  onSaved: () => void
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
      <div class="modal-card">
        <div class="modal-header">
          <h2 class="modal-title">${escapeHtml(existing ? copy.hostModal.editTitle : copy.hostModal.createTitle)}</h2>
          <button id="host-close" class="btn btn-sm">${escapeHtml(copy.common.close)}</button>
        </div>
        ${errorMsg ? `<div class="error-banner">${escapeHtml(errorMsg)}</div>` : ""}
        <form id="host-form">
          <div class="field">
            <label for="host-alias">${escapeHtml(copy.hostModal.alias)}</label>
            <input id="host-alias" class="input" required value="${escapeHtml(existing?.alias ?? "")}" placeholder="${escapeHtml(copy.hostModal.aliasPlaceholder)}" />
          </div>
          <div class="row-2col">
            <div class="field">
              <label for="host-addr">${escapeHtml(copy.hostModal.hostAddress)}</label>
              <input id="host-addr" class="input mono" required value="${escapeHtml(existing?.hostAddress ?? "")}" placeholder="${escapeHtml(copy.hostModal.hostPlaceholder)}" />
            </div>
            <div class="field">
              <label for="host-port">${escapeHtml(copy.hostModal.port)}</label>
              <input id="host-port" class="input mono" type="number" min="1" max="65535" required value="${existing?.port ?? 22}" />
            </div>
          </div>
          <div class="row-2col">
            <div class="field">
              <label for="host-user">${escapeHtml(copy.hostModal.sshUsername)}</label>
              <input id="host-user" class="input mono" required value="${escapeHtml(existing?.sshUsername ?? "ec2-user")}" placeholder="${escapeHtml(copy.hostModal.sshUserPlaceholder)}" />
            </div>
            <div class="field">
              <label for="host-env">${escapeHtml(copy.hostModal.environment)}</label>
              <select id="host-env" class="select">
                ${["Prod", "Staging", "Dev"]
                  .map(
                    (env) =>
                      `<option value="${env}" ${(existing?.environment ?? "Prod") === env ? "selected" : ""}>${env}</option>`
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
                    `<option value="${escapeHtml(k.id)}" ${existing?.sshKeyId === k.id ? "selected" : ""}>${escapeHtml(k.name)} (${escapeHtml(k.keyType)})</option>`
                )
                .join("")}
            </select>
          </div>
          <div class="field">
            <label for="host-notes">${escapeHtml(copy.hostModal.notes)}</label>
            <textarea id="host-notes" class="textarea" rows="2" placeholder="${escapeHtml(copy.hostModal.notesPlaceholder)}">${escapeHtml(existing?.notes ?? "")}</textarea>
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

    backdrop.querySelector<HTMLFormElement>("#host-form")?.addEventListener("submit", async (e) => {
      e.preventDefault();
      errorMsg = "";
      const payload = {
        id: existing?.id ?? "",
        alias: backdrop.querySelector<HTMLInputElement>("#host-alias")?.value.trim() ?? "",
        hostAddress: backdrop.querySelector<HTMLInputElement>("#host-addr")?.value.trim() ?? "",
        port: Number(backdrop.querySelector<HTMLInputElement>("#host-port")?.value ?? "22"),
        sshUsername: backdrop.querySelector<HTMLInputElement>("#host-user")?.value.trim() ?? "",
        sshKeyId: backdrop.querySelector<HTMLSelectElement>("#host-key")?.value ?? "",
        environment: backdrop.querySelector<HTMLSelectElement>("#host-env")?.value ?? "Prod",
        notes: backdrop.querySelector<HTMLTextAreaElement>("#host-notes")?.value.trim() ?? "",
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
