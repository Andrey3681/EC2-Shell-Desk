import { t } from "../i18n";
import {
  desktopApi,
  type HostInstance,
  type PrivateService,
  type SaveServiceInput,
} from "../services/desktopApi";
import { escapeHtml, extractErrorMessage } from "./dom";

interface ServicePresetMeta {
  label: string;
  protocol: "http" | "https";
  remotePort: number;
  basePath: string;
  healthPath: string;
  hintKey: string;
}

const SERVICE_PRESETS: Record<string, ServicePresetMeta> = {
  grafana: {
    label: "Grafana (3000 / 3001)",
    protocol: "http",
    remotePort: 3000,
    basePath: "",
    healthPath: "/api/health",
    hintKey: "hintGrafana",
  },
  prometheus: {
    label: "Prometheus Metrics (9090)",
    protocol: "http",
    remotePort: 9090,
    basePath: "",
    healthPath: "/-/healthy",
    hintKey: "hintPrometheus",
  },
  loki: {
    label: "Grafana Loki Logs (3100)",
    protocol: "http",
    remotePort: 3100,
    basePath: "",
    healthPath: "/ready",
    hintKey: "hintLoki",
  },
  mailpit: {
    label: "Mailpit / MailHog (8025)",
    protocol: "http",
    remotePort: 8025,
    basePath: "",
    healthPath: "/",
    hintKey: "hintMailpit",
  },
  portainer: {
    label: "Portainer Docker UI (9443 / 9000)",
    protocol: "https",
    remotePort: 9443,
    basePath: "",
    healthPath: "/api/system/status",
    hintKey: "hintPortainer",
  },
  netdata: {
    label: "Netdata Real-time Monitor (19999)",
    protocol: "http",
    remotePort: 19999,
    basePath: "",
    healthPath: "/api/v1/info",
    hintKey: "hintNetdata",
  },
  uptime_kuma: {
    label: "Uptime Kuma (3001)",
    protocol: "http",
    remotePort: 3001,
    basePath: "",
    healthPath: "/",
    hintKey: "hintUptimeKuma",
  },
  custom: {
    label: "Custom HTTP / HTTPS Service",
    protocol: "http",
    remotePort: 8080,
    basePath: "",
    healthPath: "/",
    hintKey: "hintCustom",
  },
};

export function openServiceModal(
  existing: PrivateService | null,
  hosts: HostInstance[],
  onSaved: () => void,
  defaultHostId = ""
): void {
  const copy = t();
  if (!existing && hosts.length === 0) {
    window.alert(copy.tunnels.needHostFirst);
    return;
  }

  const backdrop = document.createElement("div");
  backdrop.className = "modal-backdrop";

  let errorMsg = "";
  let preset = existing?.preset || "grafana";
  let name = existing?.name ?? "";
  let hostId = existing?.hostId || defaultHostId || hosts[0]?.id || "";
  let protocol = existing?.protocol || SERVICE_PRESETS[preset]?.protocol || "http";
  let remoteHost = existing?.remoteHost || "127.0.0.1";
  let remotePort = existing?.remotePort ?? SERVICE_PRESETS[preset]?.remotePort ?? 3000;
  let basePath = existing?.basePath ?? "";
  let localPort = existing?.localPort ?? 0;
  let healthPath = existing?.healthPath ?? SERVICE_PRESETS[preset]?.healthPath ?? "/api/health";
  let openMode = existing?.openMode || "browser";
  let allowInsecureTls = existing?.allowInsecureTls ?? false;
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
    const curCopy = t();
    const meta = SERVICE_PRESETS[preset] ?? SERVICE_PRESETS.custom;
    const hintText = (curCopy.serviceModal as unknown as Record<string, string>)[meta.hintKey] ?? "";

    backdrop.innerHTML = `
      <div class="modal-card" style="max-width:560px;">
        <div class="modal-header">
          <h2 class="modal-title">${escapeHtml(existing ? curCopy.serviceModal.editTitle : curCopy.serviceModal.createTitle)}</h2>
          <button id="svc-close" class="btn btn-sm">${escapeHtml(curCopy.common.close)}</button>
        </div>
        ${errorMsg ? `<div class="error-banner">${escapeHtml(errorMsg)}</div>` : ""}
        <form id="svc-form">
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:10px;">
            <div class="field">
              <label for="svc-preset">${escapeHtml(curCopy.serviceModal.preset)}</label>
              <select id="svc-preset" class="select">
                ${Object.entries(SERVICE_PRESETS)
                  .map(
                    ([key, p]) =>
                      `<option value="${key}" ${preset === key ? "selected" : ""}>${escapeHtml(p.label)}</option>`
                  )
                  .join("")}
              </select>
            </div>
            <div class="field">
              <label for="svc-host">${escapeHtml(curCopy.serviceModal.host)}</label>
              <select id="svc-host" class="select" required>
                ${hosts
                  .map(
                    (h) =>
                      `<option value="${escapeHtml(h.id)}" ${hostId === h.id ? "selected" : ""}>${escapeHtml(h.alias)} (${escapeHtml(h.hostAddress)})</option>`
                  )
                  .join("")}
              </select>
            </div>
          </div>

          <div class="field">
            <label for="svc-name">${escapeHtml(curCopy.serviceModal.name)}</label>
            <input id="svc-name" class="input" required maxlength="128" value="${escapeHtml(name)}" placeholder="${escapeHtml(curCopy.serviceModal.namePlaceholder)}" />
          </div>

          <div style="display:grid;grid-template-columns:110px 1fr 130px;gap:10px;">
            <div class="field">
              <label for="svc-protocol">${escapeHtml(curCopy.serviceModal.protocol)}</label>
              <select id="svc-protocol" class="select">
                <option value="http" ${protocol === "http" ? "selected" : ""}>http://</option>
                <option value="https" ${protocol === "https" ? "selected" : ""}>https://</option>
              </select>
            </div>
            <div class="field">
              <label for="svc-remote-host">${escapeHtml(curCopy.serviceModal.remoteHost)}</label>
              <input id="svc-remote-host" class="input" required value="${escapeHtml(remoteHost)}" />
            </div>
            <div class="field">
              <label for="svc-remote-port">${escapeHtml(curCopy.serviceModal.remotePort)}</label>
              <input id="svc-remote-port" type="number" min="1" max="65535" class="input" required value="${remotePort}" />
            </div>
          </div>

          <div style="display:grid;grid-template-columns:1fr 1fr;gap:10px;">
            <div class="field">
              <label for="svc-base-path">${escapeHtml(curCopy.serviceModal.basePath)}</label>
              <input id="svc-base-path" class="input" value="${escapeHtml(basePath)}" placeholder="${escapeHtml(curCopy.serviceModal.basePathPlaceholder)}" />
            </div>
            <div class="field">
              <label for="svc-local-port">${escapeHtml(curCopy.serviceModal.localPort)}</label>
              <input id="svc-local-port" type="number" min="0" max="65535" class="input" value="${localPort}" placeholder="${escapeHtml(curCopy.serviceModal.localPortPlaceholder)}" />
            </div>
          </div>

          <div style="display:grid;grid-template-columns:1fr 1fr;gap:10px;">
            <div class="field">
              <label for="svc-health-path">${escapeHtml(curCopy.serviceModal.healthPath)}</label>
              <input id="svc-health-path" class="input" value="${escapeHtml(healthPath)}" placeholder="${escapeHtml(curCopy.serviceModal.healthPathPlaceholder)}" />
            </div>
            <div class="field">
              <label for="svc-open-mode">${escapeHtml(curCopy.serviceModal.openMode)}</label>
              <select id="svc-open-mode" class="select">
                <option value="browser" ${openMode === "browser" ? "selected" : ""}>${escapeHtml(curCopy.serviceModal.openModeBrowser)}</option>
              </select>
            </div>
          </div>

          ${
            protocol === "https"
              ? `<div class="field" style="display:flex;align-items:center;gap:8px;">
                  <input id="svc-insecure-tls" type="checkbox" ${allowInsecureTls ? "checked" : ""} />
                  <label for="svc-insecure-tls" style="margin:0;cursor:pointer;">${escapeHtml(curCopy.serviceModal.allowInsecureTls)}</label>
                </div>`
              : ""
          }

          ${hintText ? `<div class="hint-box">${escapeHtml(hintText)}</div>` : ""}

          <div class="field">
            <label for="svc-notes">${escapeHtml(curCopy.serviceModal.notes)}</label>
            <input id="svc-notes" class="input" maxlength="255" value="${escapeHtml(notes)}" placeholder="${escapeHtml(curCopy.serviceModal.notesPlaceholder)}" />
          </div>

          <div class="modal-footer">
            <button type="button" id="svc-cancel" class="btn">${escapeHtml(curCopy.common.cancel)}</button>
            <button type="submit" class="btn btn-primary">${escapeHtml(curCopy.common.save)}</button>
          </div>
        </form>
      </div>
    `;

    backdrop.querySelector("#svc-close")?.addEventListener("click", closeModal);
    backdrop.querySelector("#svc-cancel")?.addEventListener("click", closeModal);

    const captureDraft = () => {
      name = backdrop.querySelector<HTMLInputElement>("#svc-name")?.value ?? name;
      hostId = backdrop.querySelector<HTMLSelectElement>("#svc-host")?.value ?? hostId;
      remoteHost = backdrop.querySelector<HTMLInputElement>("#svc-remote-host")?.value ?? remoteHost;
      const rp = parseInt(backdrop.querySelector<HTMLInputElement>("#svc-remote-port")?.value ?? "", 10);
      if (Number.isFinite(rp)) remotePort = rp;
      basePath = backdrop.querySelector<HTMLInputElement>("#svc-base-path")?.value ?? basePath;
      const lp = parseInt(backdrop.querySelector<HTMLInputElement>("#svc-local-port")?.value ?? "", 10);
      if (Number.isFinite(lp)) localPort = lp;
      healthPath = backdrop.querySelector<HTMLInputElement>("#svc-health-path")?.value ?? healthPath;
      openMode = (backdrop.querySelector<HTMLSelectElement>("#svc-open-mode")?.value ?? openMode) as any;
      notes = backdrop.querySelector<HTMLInputElement>("#svc-notes")?.value ?? notes;
    };

    const presetEl = backdrop.querySelector<HTMLSelectElement>("#svc-preset");
    presetEl?.addEventListener("change", (e) => {
      captureDraft();
      const selected = (e.target as HTMLSelectElement).value;
      preset = selected as any;
      const pMeta = SERVICE_PRESETS[selected];
      if (pMeta) {
        if (!name || Object.values(SERVICE_PRESETS).some((x) => x.label.startsWith(name))) {
          name = pMeta.label.split(" (")[0];
        }
        protocol = pMeta.protocol;
        if (protocol !== "https") {
          allowInsecureTls = false;
        }
        remotePort = pMeta.remotePort;
        healthPath = pMeta.healthPath;
      }
      render();
    });

    const protocolEl = backdrop.querySelector<HTMLSelectElement>("#svc-protocol");
    protocolEl?.addEventListener("change", (e) => {
      captureDraft();
      protocol = (e.target as HTMLSelectElement).value as any;
      if (protocol !== "https") {
        allowInsecureTls = false;
      }
      render();
    });

    const insecureTlsEl = backdrop.querySelector<HTMLInputElement>("#svc-insecure-tls");
    insecureTlsEl?.addEventListener("change", (e) => {
      allowInsecureTls = (e.target as HTMLInputElement).checked;
    });

    const formEl = backdrop.querySelector<HTMLFormElement>("#svc-form");
    formEl?.addEventListener("submit", async (e) => {
      e.preventDefault();
      errorMsg = "";

      const nameVal = (backdrop.querySelector<HTMLInputElement>("#svc-name")?.value ?? "").trim();
      const hostIdVal = backdrop.querySelector<HTMLSelectElement>("#svc-host")?.value ?? "";
      const protocolVal = (backdrop.querySelector<HTMLSelectElement>("#svc-protocol")?.value ?? "http") as any;
      const remoteHostVal = (backdrop.querySelector<HTMLInputElement>("#svc-remote-host")?.value ?? "").trim();
      const remotePortVal = parseInt(backdrop.querySelector<HTMLInputElement>("#svc-remote-port")?.value ?? "3000", 10);
      const basePathVal = (backdrop.querySelector<HTMLInputElement>("#svc-base-path")?.value ?? "").trim();
      const localPortVal = parseInt(backdrop.querySelector<HTMLInputElement>("#svc-local-port")?.value ?? "0", 10) || 0;
      const healthPathVal = (backdrop.querySelector<HTMLInputElement>("#svc-health-path")?.value ?? "").trim();
      const openModeVal = (backdrop.querySelector<HTMLSelectElement>("#svc-open-mode")?.value ?? "browser") as any;
      const allowInsecureTlsVal =
        protocolVal === "https" &&
        Boolean(backdrop.querySelector<HTMLInputElement>("#svc-insecure-tls")?.checked);
      const notesVal = (backdrop.querySelector<HTMLInputElement>("#svc-notes")?.value ?? "").trim();

      const payload: SaveServiceInput = {
        hostId: hostIdVal,
        name: nameVal,
        preset: preset as any,
        protocol: protocolVal,
        remoteHost: remoteHostVal,
        remotePort: remotePortVal,
        basePath: basePathVal,
        localPort: localPortVal,
        healthPath: healthPathVal,
        openMode: openModeVal,
        allowInsecureTls: allowInsecureTlsVal,
        notes: notesVal,
      };

      if (existing) {
        payload.id = existing.id;
      }

      try {
        if (existing) {
          await desktopApi.services.update(payload);
        } else {
          await desktopApi.services.create(payload);
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
