import { t } from "../i18n";
import {
  desktopApi,
  type HostInstance,
  type SSHTunnel,
} from "../services/desktopApi";
import { escapeHtml, extractErrorMessage } from "./dom";

interface PresetDefaults {
  label: string;
  remotePort: number;
  localPort: number;
  hintKey: string;
}

const PRESETS: Record<string, PresetDefaults> = {
  postgresql: {
    label: "PostgreSQL / AWS RDS Postgres (5432)",
    remotePort: 5432,
    localPort: 15432,
    hintKey: "hintPostgres",
  },
  mysql: {
    label: "MySQL / MariaDB / Aurora (3306)",
    remotePort: 3306,
    localPort: 13306,
    hintKey: "hintMysql",
  },
  mongodb: {
    label: "MongoDB / AWS DocumentDB (27017)",
    remotePort: 27017,
    localPort: 27017,
    hintKey: "hintMongo",
  },
  redis: {
    label: "Redis / AWS ElastiCache (6379)",
    remotePort: 6379,
    localPort: 16379,
    hintKey: "hintRedis",
  },
  sqlserver: {
    label: "SQL Server / RDS SQL Server (1433)",
    remotePort: 1433,
    localPort: 11433,
    hintKey: "hintSqlServer",
  },
  custom: {
    label: "Custom TCP Port",
    remotePort: 8080,
    localPort: 18080,
    hintKey: "hintCustom",
  },
};

export function openTunnelModal(
  existing: SSHTunnel | null,
  hosts: HostInstance[],
  onSaved: () => void
): void {
  const copy = t();
  if (!existing && hosts.length === 0) {
    window.alert(copy.tunnels.needHostFirst);
    return;
  }

  const backdrop = document.createElement("div");
  backdrop.className = "modal-backdrop";

  let errorMsg = "";
  let currentPreset = existing?.preset || "postgresql";
  let currentName = existing?.name ?? "";
  let currentHostId = existing?.hostId || hosts[0]?.id || "";
  let currentRemoteHost = existing?.remoteHost ?? "";
  let currentRemotePort =
    existing?.remotePort ?? PRESETS[currentPreset]?.remotePort ?? 5432;
  let currentLocalHost = existing?.localHost || "127.0.0.1";
  let currentLocalPort =
    existing?.localPort ?? PRESETS[currentPreset]?.localPort ?? 15432;
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

  const render = () => {
    const curCopy = t();
    const presetMeta = PRESETS[currentPreset] ?? PRESETS.custom;
    const hintText = curCopy.tunnelModal[presetMeta.hintKey] ?? "";

    backdrop.innerHTML = `
      <div class="modal-card">
        <div class="modal-header">
          <h2 class="modal-title">${escapeHtml(existing ? curCopy.tunnelModal.editTitle : curCopy.tunnelModal.createTitle)}</h2>
          <button id="tun-close" class="btn btn-sm">${escapeHtml(curCopy.common.close)}</button>
        </div>
        ${errorMsg ? `<div class="error-banner">${escapeHtml(errorMsg)}</div>` : ""}
        <form id="tun-form">
          <div class="row-2col">
            <div class="field">
              <label for="tun-name">${escapeHtml(curCopy.tunnelModal.name)}</label>
              <input id="tun-name" class="input" required value="${escapeHtml(currentName)}" placeholder="${escapeHtml(curCopy.tunnelModal.namePlaceholder)}" />
            </div>
            <div class="field">
              <label for="tun-preset">${escapeHtml(curCopy.tunnelModal.preset)}</label>
              <select id="tun-preset" class="select">
                ${Object.entries(PRESETS)
                  .map(
                    ([key, meta]) =>
                      `<option value="${key}" ${currentPreset === key ? "selected" : ""}>${escapeHtml(meta.label)}</option>`
                  )
                  .join("")}
              </select>
            </div>
          </div>

          <div class="hint-box">${escapeHtml(hintText)}</div>

          <div class="field">
            <label for="tun-host">${escapeHtml(curCopy.tunnelModal.bastionHost)}</label>
            <select id="tun-host" class="select" required>
              ${hosts
                .map(
                  (h) =>
                    `<option value="${escapeHtml(h.id)}" ${currentHostId === h.id ? "selected" : ""}>${escapeHtml(h.alias)} (${escapeHtml(h.sshUsername)}@${escapeHtml(h.hostAddress)})</option>`
                )
                .join("")}
            </select>
          </div>

          <div class="row-2col">
            <div class="field">
              <label for="tun-remote-host">${escapeHtml(curCopy.tunnelModal.remoteHost)}</label>
              <input id="tun-remote-host" class="input mono" required value="${escapeHtml(currentRemoteHost)}" placeholder="${escapeHtml(curCopy.tunnelModal.remoteHostPlaceholder)}" />
            </div>
            <div class="field">
              <label for="tun-remote-port">${escapeHtml(curCopy.tunnelModal.remotePort)}</label>
              <input id="tun-remote-port" class="input mono" type="number" min="1" max="65535" required value="${currentRemotePort}" />
            </div>
          </div>

          <div class="row-2col">
            <div class="field">
              <label for="tun-local-host">${escapeHtml(curCopy.tunnelModal.localHost)}</label>
              <select id="tun-local-host" class="select">
                <option value="127.0.0.1" ${currentLocalHost === "127.0.0.1" ? "selected" : ""}>${escapeHtml(curCopy.tunnelModal.bindLocalOnly)}</option>
                <option value="0.0.0.0" ${currentLocalHost === "0.0.0.0" ? "selected" : ""}>${escapeHtml(curCopy.tunnelModal.bindDocker)}</option>
              </select>
            </div>
            <div class="field">
              <label for="tun-local-port">${escapeHtml(curCopy.tunnelModal.localPort)}</label>
              <input id="tun-local-port" class="input mono" type="number" min="1" max="65535" required value="${currentLocalPort}" />
            </div>
          </div>

          <div class="field">
            <label for="tun-notes">${escapeHtml(curCopy.tunnelModal.notes)}</label>
            <input id="tun-notes" class="input" value="${escapeHtml(currentNotes)}" placeholder="${escapeHtml(curCopy.tunnelModal.notesPlaceholder)}" />
          </div>

          <div class="modal-footer">
            <button type="button" id="tun-cancel" class="btn">${escapeHtml(curCopy.common.cancel)}</button>
            <button type="submit" class="btn btn-primary">${escapeHtml(curCopy.common.save)}</button>
          </div>
        </form>
      </div>
    `;

    backdrop.querySelector("#tun-close")?.addEventListener("click", closeModal);
    backdrop.querySelector("#tun-cancel")?.addEventListener("click", closeModal);

    const syncInputs = () => {
      currentName = backdrop.querySelector<HTMLInputElement>("#tun-name")?.value ?? "";
      currentHostId = backdrop.querySelector<HTMLSelectElement>("#tun-host")?.value ?? "";
      currentRemoteHost =
        backdrop.querySelector<HTMLInputElement>("#tun-remote-host")?.value ?? "";
      currentRemotePort = Number(
        backdrop.querySelector<HTMLInputElement>("#tun-remote-port")?.value ?? "5432"
      );
      currentLocalHost =
        backdrop.querySelector<HTMLSelectElement>("#tun-local-host")?.value ?? "127.0.0.1";
      currentLocalPort = Number(
        backdrop.querySelector<HTMLInputElement>("#tun-local-port")?.value ?? "15432"
      );
      currentNotes = backdrop.querySelector<HTMLInputElement>("#tun-notes")?.value ?? "";
    };

    backdrop.querySelector<HTMLSelectElement>("#tun-preset")?.addEventListener("change", (e) => {
      syncInputs();
      const nextPreset = (e.target as HTMLSelectElement).value;
      currentPreset = nextPreset;
      const meta = PRESETS[nextPreset] ?? PRESETS.custom;
      if (!existing) {
        currentRemotePort = meta.remotePort;
        currentLocalPort = meta.localPort;
      }
      render();
    });

    backdrop.querySelector<HTMLFormElement>("#tun-form")?.addEventListener("submit", async (e) => {
      e.preventDefault();
      syncInputs();
      errorMsg = "";

      const payload = {
        id: existing?.id ?? "",
        hostId: currentHostId.trim(),
        name: currentName.trim(),
        preset: currentPreset,
        localHost: currentLocalHost,
        localPort: currentLocalPort,
        remoteHost: currentRemoteHost.trim(),
        remotePort: currentRemotePort,
        notes: currentNotes.trim(),
      };

      try {
        if (existing) {
          await desktopApi.tunnels.update(payload);
        } else {
          await desktopApi.tunnels.create(payload);
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
