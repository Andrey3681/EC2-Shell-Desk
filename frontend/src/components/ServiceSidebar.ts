import { t } from "../i18n";
import type {
  HostInstance,
  PrivateService,
  ServiceRuntimeStatus,
} from "../services/desktopApi";
import { escapeHtml } from "./dom";

export interface ServiceSidebarState {
  services: PrivateService[];
  hosts: HostInstance[];
  statuses: Map<string, ServiceRuntimeStatus>;
  search: string;
  hostFilter: string;
  activeHostId: string;
}

export interface ServiceSidebarCallbacks {
  onSearchChange: (q: string) => void;
  onHostFilterChange: (hostId: string) => void;
  onNewService: () => void;
  onOpenService: (svc: PrivateService) => void;
  onStartService: (svc: PrivateService) => void;
  onStopService: (svc: PrivateService) => void;
  onProbeHealth: (svc: PrivateService) => void;
  onEditService: (svc: PrivateService) => void;
  onDeleteService: (svc: PrivateService) => void;
}

export function renderServiceSidebarBody(
  container: HTMLElement,
  state: ServiceSidebarState,
  cb: ServiceSidebarCallbacks
): void {
  const copy = t();
  const hostNames = new Map(state.hosts.map((h) => [h.id, h.alias]));
  const query = state.search.trim().toLowerCase();
  const activeHostAlias = state.activeHostId ? (hostNames.get(state.activeHostId) ?? "") : "";

  const filtered = state.services.filter((svc) => {
    if (state.hostFilter === "ACTIVE") {
      if (!state.activeHostId || svc.hostId !== state.activeHostId) return false;
    } else if (state.hostFilter !== "ALL") {
      if (svc.hostId !== state.hostFilter) return false;
    }

    if (!query) return true;
    const hName = hostNames.get(svc.hostId) ?? "";
    return (
      svc.name.toLowerCase().includes(query) ||
      svc.remoteHost.toLowerCase().includes(query) ||
      String(svc.remotePort).includes(query) ||
      svc.notes.toLowerCase().includes(query) ||
      hName.toLowerCase().includes(query)
    );
  });

  const statusBadge = (st: ServiceRuntimeStatus | undefined) => {
    const s = st?.state ?? "stopped";
    if (s === "active") {
      return `<span class="badge badge-dev">${escapeHtml(copy.services.stateActive)}</span>`;
    }
    if (s === "connecting") {
      return `<span class="badge badge-staging">${escapeHtml(copy.services.stateConnecting)}</span>`;
    }
    if (s === "error") {
      return `<span class="badge badge-prod">${escapeHtml(copy.services.stateError)}</span>`;
    }
    return `<span class="badge" style="background:rgba(148,163,184,0.18);color:var(--text-secondary);">${escapeHtml(copy.services.stateStopped)}</span>`;
  };

  const healthBadge = (st: ServiceRuntimeStatus | undefined) => {
    if (!st || st.state !== "active") return "";
    const h = st.healthStatus;
    if (h === "healthy") {
      const code = st.httpStatus ? `${st.httpStatus} OK` : copy.services.healthHealthy;
      const lat = st.latencyMs ? ` · ${st.latencyMs}ms` : "";
      return `<span class="badge badge-dev" title="${code}${lat}">&#10003; ${escapeHtml(code)}${escapeHtml(lat)}</span>`;
    }
    if (h === "degraded") {
      const code = st.httpStatus ? `HTTP ${st.httpStatus}` : copy.services.healthDegraded;
      return `<span class="badge badge-staging" title="${code}">&#9888; ${escapeHtml(code)}</span>`;
    }
    if (h === "unhealthy") {
      return `<span class="badge badge-prod" title="${escapeHtml(st.errorMessage || "")}">&#10007; ${escapeHtml(copy.services.healthUnhealthy)}</span>`;
    }
    return `<span class="badge" style="background:rgba(148,163,184,0.18);color:var(--text-secondary);">${escapeHtml(copy.services.healthUnknown)}</span>`;
  };

  container.innerHTML = `
    <div class="sidebar-header">
      <span>${escapeHtml(copy.services.title)}</span>
      <button id="btn-new-service" class="btn btn-primary btn-sm">${escapeHtml(copy.services.newService)}</button>
    </div>
    <div class="sidebar-filters">
      <input
        id="service-search"
        class="input"
        placeholder="${escapeHtml(copy.services.searchPlaceholder)}"
        value="${escapeHtml(state.search)}"
      />
      <select id="service-host-filter" class="select">
        <option value="ALL" ${state.hostFilter === "ALL" ? "selected" : ""}>${escapeHtml(copy.services.allHosts)}</option>
        ${
          state.activeHostId
            ? `<option value="ACTIVE" ${state.hostFilter === "ACTIVE" ? "selected" : ""}>${escapeHtml(copy.services.activeHost)}: ${escapeHtml(activeHostAlias)}</option>`
            : ""
        }
        ${state.hosts
          .map(
            (h) =>
              `<option value="${escapeHtml(h.id)}" ${state.hostFilter === h.id ? "selected" : ""}>${escapeHtml(h.alias)} (${escapeHtml(h.hostAddress)})</option>`
          )
          .join("")}
      </select>
    </div>
    <div class="host-list">
      ${
        state.services.length === 0
          ? `<div style="padding:16px;color:var(--text-secondary);text-align:center;line-height:1.45;">${escapeHtml(copy.services.emptyState)}</div>`
          : filtered.length === 0
            ? `<div style="padding:16px;color:var(--text-secondary);text-align:center;line-height:1.45;">${escapeHtml(copy.services.emptyFiltered)}</div>`
            : filtered
                .map((svc) => {
                  const st = state.statuses.get(svc.id);
                  const isRunning = st?.state === "active" || st?.state === "connecting";
                  const hostAlias = hostNames.get(svc.hostId) ?? "-";
                  const remoteTarget = `${svc.protocol}://${svc.remoteHost}:${svc.remotePort}${svc.basePath || ""}`;
                  const localDisplay = st?.localUrl || `127.0.0.1:${svc.localPort || "auto"}${svc.basePath || ""}`;

                  return `
                    <div class="host-item" data-svc-id="${escapeHtml(svc.id)}">
                      <div class="host-item-top">
                        <span style="font-weight:600;">${escapeHtml(svc.name)}</span>
                        <div style="display:flex;gap:4px;align-items:center;">
                          ${healthBadge(st)}
                          ${statusBadge(st)}
                        </div>
                      </div>
                      <div class="tunnel-route mono" style="font-size:11px;">${escapeHtml(localDisplay)} &rarr; ${escapeHtml(remoteTarget)}</div>
                      <div style="margin-top:4px;font-size:11px;color:var(--text-secondary);">
                        VM: <strong>${escapeHtml(hostAlias)}</strong> · <span class="mono">${escapeHtml(svc.preset)}</span>
                        ${svc.notes ? ` · <span>${escapeHtml(svc.notes)}</span>` : ""}
                      </div>
                      ${
                        st?.state === "error" && st.errorMessage
                          ? `<div style="margin-top:4px;font-size:11px;color:#fca5a5;">${escapeHtml(st.errorMessage)}</div>`
                          : ""
                      }
                      <div class="host-item-actions" style="flex-wrap:wrap;margin-top:6px;">
                        <button class="btn btn-primary btn-sm" data-svc-action="open" data-id="${escapeHtml(svc.id)}">&#127760; ${escapeHtml(copy.services.openInBrowser)}</button>
                        ${
                          isRunning
                            ? `<button class="btn btn-danger btn-sm" data-svc-action="stop" data-id="${escapeHtml(svc.id)}">${escapeHtml(copy.services.stop)}</button>`
                            : `<button class="btn btn-success btn-sm" data-svc-action="start" data-id="${escapeHtml(svc.id)}">${escapeHtml(copy.services.start)}</button>`
                        }
                        ${
                          isRunning && st?.localUrl
                            ? `<button class="btn btn-sm" data-svc-copy="${escapeHtml(st.localUrl)}">${escapeHtml(copy.services.copyUrl)}</button>`
                            : ""
                        }
                        <button class="btn btn-sm" data-svc-action="edit" data-id="${escapeHtml(svc.id)}" ${isRunning ? "disabled" : ""}>${escapeHtml(copy.common.edit)}</button>
                        <button class="btn btn-danger btn-sm" data-svc-action="delete" data-id="${escapeHtml(svc.id)}" ${isRunning ? "disabled" : ""}>${escapeHtml(copy.common.delete)}</button>
                      </div>
                    </div>
                  `;
                })
                .join("")
      }
    </div>
  `;

  container.querySelector("#btn-new-service")?.addEventListener("click", cb.onNewService);

  container.querySelector<HTMLInputElement>("#service-search")?.addEventListener("input", (e) => {
    cb.onSearchChange((e.target as HTMLInputElement).value);
  });

  container
    .querySelector<HTMLSelectElement>("#service-host-filter")
    ?.addEventListener("change", (e) => {
      cb.onHostFilterChange((e.target as HTMLSelectElement).value);
    });

  const byId = new Map(state.services.map((item) => [item.id, item]));

  container.querySelectorAll<HTMLButtonElement>("button[data-svc-action]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const svc = byId.get(btn.dataset.id ?? "");
      if (!svc) return;
      const action = btn.dataset.svcAction;
      if (action === "open") cb.onOpenService(svc);
      if (action === "start") cb.onStartService(svc);
      if (action === "stop") cb.onStopService(svc);
      if (action === "edit") cb.onEditService(svc);
      if (action === "delete") cb.onDeleteService(svc);
    });
  });

  container.querySelectorAll<HTMLButtonElement>("button[data-svc-copy]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const url = btn.dataset.svcCopy ?? "";
      if (!url) return;
      try {
        await navigator.clipboard.writeText(url);
        const orig = btn.textContent ?? "";
        btn.textContent = copy.services.copied;
        setTimeout(() => {
          btn.textContent = orig;
        }, 1200);
      } catch {
        window.prompt("URL:", url);
      }
    });
  });
}
