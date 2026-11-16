import { t } from "../i18n";
import {
  desktopApi,
  type KnownHost,
  type SSHKeySummary,
} from "../services/desktopApi";
import { escapeHtml, extractErrorMessage } from "./dom";

export function openKeyVaultModal(onChanged: () => void): void {
  const backdrop = document.createElement("div");
  backdrop.className = "modal-backdrop";

  let activeTab: "keys" | "known" = "keys";
  let keys: SSHKeySummary[] = [];
  let knownHosts: KnownHost[] = [];
  let selectedPath = "";
  let errorMsg = "";

  const closeModal = () => {
    window.removeEventListener("keydown", onKeyDown);
    backdrop.remove();
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Escape") {
      closeModal();
    }
  };
  window.addEventListener("keydown", onKeyDown);
  backdrop.addEventListener("click", (e) => {
    if (e.target === backdrop) {
      closeModal();
    }
  });

  const loadData = async () => {
    try {
      const [loadedKeys, loadedKnown] = await Promise.all([
        desktopApi.keys.list(),
        desktopApi.terminal.listKnownHosts(),
      ]);
      keys = loadedKeys ?? [];
      knownHosts = loadedKnown ?? [];
    } catch (err) {
      errorMsg = extractErrorMessage(err);
    }
    render();
  };

  const render = () => {
    const copy = t();
    backdrop.innerHTML = `
      <div class="modal-card" style="max-width:680px;">
        <div class="modal-header">
          <h2 class="modal-title">${escapeHtml(copy.vaultModal.title)}</h2>
          <button id="vault-close" class="btn btn-sm">${escapeHtml(copy.common.close)}</button>
        </div>
        <div style="display:flex;gap:8px;margin-bottom:12px;">
          <button id="tab-keys" class="btn btn-sm ${activeTab === "keys" ? "btn-primary" : ""}">${escapeHtml(copy.vaultModal.tabKeys)} (${keys.length})</button>
          <button id="tab-known" class="btn btn-sm ${activeTab === "known" ? "btn-primary" : ""}">${escapeHtml(copy.vaultModal.tabKnownHosts)} (${knownHosts.length})</button>
        </div>
        ${errorMsg ? `<div class="error-banner">${escapeHtml(errorMsg)}</div>` : ""}
        ${
          activeTab === "keys"
            ? `
          <form id="import-key-form" style="padding:12px;border:1px solid var(--border-subtle);border-radius:8px;margin-bottom:14px;background:var(--bg-input);">
            <div class="row-2col">
              <div class="field">
                <label for="key-name">${escapeHtml(copy.vaultModal.keyName)}</label>
                <input id="key-name" class="input" required placeholder="${escapeHtml(copy.vaultModal.keyNamePlaceholder)}" />
              </div>
              <div class="field">
                <label for="key-passphrase">${escapeHtml(copy.vaultModal.passphrase)}</label>
                <input id="key-passphrase" class="input" type="password" />
              </div>
            </div>
            <div style="display:flex;align-items:center;gap:8px;margin-bottom:10px;">
              <button type="button" id="btn-pick-key" class="btn btn-sm">${escapeHtml(copy.vaultModal.pickFile)}</button>
              <span id="picked-path" class="mono" style="font-size:11px;color:var(--text-secondary);overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">${escapeHtml(selectedPath)}</span>
            </div>
            <div class="field">
              <label for="key-pem">${escapeHtml(copy.vaultModal.orPastePEM)}</label>
              <textarea id="key-pem" class="textarea mono" rows="3" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"></textarea>
            </div>
            <button type="submit" class="btn btn-primary btn-sm">${escapeHtml(copy.vaultModal.importKey)}</button>
          </form>
          <div style="display:flex;flex-direction:column;gap:6px;">
            ${
              keys.length === 0
                ? `<div style="color:var(--text-secondary);">${escapeHtml(copy.vaultModal.noKeys)}</div>`
                : keys
                    .map(
                      (k) => `
                <div style="display:flex;align-items:center;justify-content:space-between;padding:8px 10px;border:1px solid var(--border-subtle);border-radius:6px;background:var(--bg-card);">
                  <div>
                    <div style="font-weight:600;">${escapeHtml(k.name)} <span class="badge badge-dev">${escapeHtml(k.keyType)}</span> ${k.hasPassphrase ? `<span class="badge badge-staging">${escapeHtml(copy.vaultModal.protectedBadge)}</span>` : ""}</div>
                    <div class="mono" style="font-size:11px;color:var(--text-secondary);">${escapeHtml(k.fingerprint)}</div>
                  </div>
                  <button class="btn btn-danger btn-sm" data-del-key="${escapeHtml(k.id)}">${escapeHtml(copy.common.delete)}</button>
                </div>`
                    )
                    .join("")
            }
          </div>`
            : `
          <div style="display:flex;flex-direction:column;gap:6px;">
            ${
              knownHosts.length === 0
                ? `<div style="color:var(--text-secondary);">${escapeHtml(copy.vaultModal.noKnownHosts)}</div>`
                : knownHosts
                    .map(
                      (kh) => `
                <div style="display:flex;align-items:center;justify-content:space-between;padding:8px 10px;border:1px solid var(--border-subtle);border-radius:6px;background:var(--bg-card);">
                  <div>
                    <div class="mono" style="font-weight:600;">${escapeHtml(kh.hostAddress)}:${kh.port} (${escapeHtml(kh.keyType)})</div>
                    <div class="mono" style="font-size:11px;color:var(--text-secondary);">${escapeHtml(kh.fingerprint)}</div>
                  </div>
                  <button class="btn btn-danger btn-sm" data-del-kh="${escapeHtml(kh.id)}">${escapeHtml(copy.vaultModal.revoke)}</button>
                </div>`
                    )
                    .join("")
            }
          </div>`
        }
      </div>
    `;

    backdrop.querySelector("#vault-close")?.addEventListener("click", closeModal);
    backdrop.querySelector("#tab-keys")?.addEventListener("click", () => {
      activeTab = "keys";
      render();
    });
    backdrop.querySelector("#tab-known")?.addEventListener("click", () => {
      activeTab = "known";
      render();
    });

    backdrop.querySelector("#btn-pick-key")?.addEventListener("click", async () => {
      try {
        const picked = await desktopApi.keys.pickFile();
        if (picked) {
          selectedPath = picked;
          const nameEl = backdrop.querySelector<HTMLInputElement>("#key-name");
          if (nameEl && !nameEl.value.trim()) {
            const parts = picked.split(/[\\/]/);
            nameEl.value = parts[parts.length - 1] ?? "";
          }
          const pathLabel = backdrop.querySelector("#picked-path");
          if (pathLabel) pathLabel.textContent = picked;
        }
      } catch (err) {
        errorMsg = extractErrorMessage(err);
        render();
      }
    });

    backdrop.querySelector<HTMLFormElement>("#import-key-form")?.addEventListener("submit", async (e) => {
      e.preventDefault();
      errorMsg = "";
      const name = backdrop.querySelector<HTMLInputElement>("#key-name")?.value.trim() ?? "";
      const passphrase = backdrop.querySelector<HTMLInputElement>("#key-passphrase")?.value ?? "";
      const pasted = backdrop.querySelector<HTMLTextAreaElement>("#key-pem")?.value.trim() ?? "";

      try {
        if (selectedPath && !pasted) {
          await desktopApi.keys.importFromFile(name, selectedPath, passphrase);
        } else {
          await desktopApi.keys.importContent(name, pasted, passphrase);
        }
        selectedPath = "";
        await loadData();
        onChanged();
      } catch (err) {
        errorMsg = extractErrorMessage(err);
        render();
      }
    });

    backdrop.querySelectorAll<HTMLButtonElement>("button[data-del-key]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        errorMsg = "";
        try {
          await desktopApi.keys.delete(btn.dataset.delKey ?? "");
          await loadData();
          onChanged();
        } catch (err) {
          errorMsg = extractErrorMessage(err);
          render();
        }
      });
    });

    backdrop.querySelectorAll<HTMLButtonElement>("button[data-del-kh]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        errorMsg = "";
        try {
          await desktopApi.terminal.removeKnownHost(btn.dataset.delKh ?? "");
          await loadData();
        } catch (err) {
          errorMsg = extractErrorMessage(err);
          render();
        }
      });
    });
  };

  render();
  document.body.appendChild(backdrop);
  void loadData();
}
