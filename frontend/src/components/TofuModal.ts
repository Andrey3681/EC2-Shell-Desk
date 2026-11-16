import { t } from "../i18n";
import { desktopApi, type TOFUChallengeEvent } from "../services/desktopApi";
import { escapeHtml } from "./dom";

export function openTofuModal(challenge: TOFUChallengeEvent): void {
  const copy = t();
  const backdrop = document.createElement("div");
  backdrop.className = "modal-backdrop";

  backdrop.innerHTML = `
    <div class="modal-card" style="max-width:480px;">
      <div class="modal-header">
        <h2 class="modal-title">${escapeHtml(copy.tofu.title)}</h2>
      </div>
      <p style="color:var(--text-secondary);line-height:1.45;margin-top:0;">${escapeHtml(copy.tofu.warning)}</p>
      <div style="padding:10px;border-radius:8px;background:var(--bg-input);border:1px solid var(--border-subtle);margin-bottom:14px;">
        <div><strong>${escapeHtml(copy.tofu.host)}:</strong> <span class="mono">${escapeHtml(challenge.hostAddress)}:${challenge.port}</span></div>
        <div style="margin-top:4px;"><strong>${escapeHtml(copy.tofu.algorithm)}:</strong> <span class="mono">${escapeHtml(challenge.keyType)}</span></div>
        <div style="margin-top:4px;"><strong>${escapeHtml(copy.tofu.fingerprint)}:</strong><div class="mono" style="color:var(--accent-warn);word-break:break-all;">${escapeHtml(challenge.fingerprint)}</div></div>
      </div>
      <div class="modal-footer">
        <button id="tofu-reject" class="btn btn-danger">${escapeHtml(copy.tofu.reject)}</button>
        <button id="tofu-trust" class="btn btn-success">${escapeHtml(copy.tofu.trustAndConnect)}</button>
      </div>
    </div>
  `;

  const decide = async (trusted: boolean) => {
    backdrop.remove();
    await desktopApi.terminal.respondTOFU(challenge.requestId, trusted);
  };

  backdrop.querySelector("#tofu-reject")?.addEventListener("click", () => {
    void decide(false);
  });
  backdrop.querySelector("#tofu-trust")?.addEventListener("click", () => {
    void decide(true);
  });

  document.body.appendChild(backdrop);
}
