import { t } from "../i18n";
import type {
  HostInstance,
  SSHTunnel,
  TunnelRuntimeStatus,
} from "../services/desktopApi";
import { escapeHtml, formatBytes } from "./dom";

export interface TunnelSidebarState {
  tunnels: SSHTunnel[];
  hosts: HostInstance[];
  statuses: Map<string, TunnelRuntimeStatus>;
}

export interface TunnelSidebarCallbacks {
  onNewTunnel: () => void;
  onStartTunnel: (tunnel: SSHTunnel) => void;
  onStopTunnel: (tunnel: SSHTunnel) => void;
  onEditTunnel: (tunnel: SSHTunnel) => void;
  onDeleteTunnel: (tunnel: SSHTunnel) => void;
}

export function renderTunnelSidebarBody(
  container: HTMLElement,
  state: TunnelSidebarState,
  cb: TunnelSidebarCallbacks
): void {
  const copy = t();
  const hostNames = new Map(state.hosts.map((h) => [h.id, h.alias]));

  const statusBadge = (st: TunnelRuntimeStatus | undefined) => {
    const s = st?.state ?? "stopped";
    if (s === "active") {
      return `<span class="badge badge-dev">${escapeHtml(copy.tunnels.stateActive)}</span>`;
    }
    if (s === "connecting") {
      return `<span class="badge badge-staging">${escapeHtml(copy.tunnels.stateConnecting)}</span>`;
    }
    if (s === "error") {
      return `<span class="badge badge-prod">${escapeHtml(copy.tunnels.stateError)}</span>`;
    }
    return `<span class="badge" style="background:rgba(148,163,184,0.18);color:var(--text-secondary);">${escapeHtml(copy.tunnels.stateStopped)}</span>`;
  };

  container.innerHTML = `
    <div class="sidebar-header">
      <span>${escapeHtml(copy.tunnels.title)}</span>
      <button id="btn-new-tunnel" class="btn btn-primary btn-sm">${escapeHtml(copy.tunnels.newTunnel)}</button>
    </div>
    <div class="host-list">
      ${
        state.tunnels.length === 0
          ? `<div style="padding:16px;color:var(--text-secondary);text-align:center;line-height:1.45;">${escapeHtml(copy.tunnels.emptyState)}</div>`
          : state.tunnels
              .map((tun) => {
                const st = state.statuses.get(tun.id);
                const isRunning = st?.state === "active" || st?.state === "connecting";
                const bastion = hostNames.get(tun.hostId) ?? "-";
                const localAddr = `${tun.localHost || "127.0.0.1"}:${tun.localPort}`;
                const copyAddr =
                  tun.localHost === "0.0.0.0"
                    ? `host.docker.internal:${tun.localPort}`
                    : localAddr;
                const remoteAddr = `${tun.remoteHost}:${tun.remotePort}`;
                const trafficLine =
                  st?.state === "active"
                    ? `<div style="margin-top:4px;font-size:11px;color:var(--accent-success);" class="mono">${st.activeConnections} ${escapeHtml(copy.tunnels.connections)} · &darr;${escapeHtml(formatBytes(st.bytesIn))} &uarr;${escapeHtml(formatBytes(st.bytesOut))}</div>`
                    : st?.state === "error" && st.errorMessage
                      ? `<div style="margin-top:4px;font-size:11px;color:#fca5a5;">${escapeHtml(st.errorMessage)}</div>`
                      : "";

                return `
                  <div class="host-item" data-tun-id="${escapeHtml(tun.id)}">
                    <div class="host-item-top">
                      <span>${escapeHtml(tun.name)}</span>
                      ${statusBadge(st)}
                    </div>
                    <div class="tunnel-route">${escapeHtml(localAddr)} &rarr; ${escapeHtml(remoteAddr)}</div>
                    <div style="margin-top:4px;font-size:11px;color:var(--text-secondary);">
                      ${escapeHtml(copy.tunnels.bastion)}: <strong>${escapeHtml(bastion)}</strong> · <span class="mono">${escapeHtml(tun.preset)}</span>
                    </div>
                    ${trafficLine}
                    <div class="host-item-actions" style="flex-wrap:wrap;">
                      ${
                        isRunning
                          ? `<button class="btn btn-danger btn-sm" data-tun-action="stop" data-id="${escapeHtml(tun.id)}">${escapeHtml(copy.tunnels.stop)}</button>`
                          : `<button class="btn btn-success btn-sm" data-tun-action="start" data-id="${escapeHtml(tun.id)}">${escapeHtml(copy.tunnels.start)}</button>`
                      }
                      <button class="btn btn-sm" data-tun-copy="${escapeHtml(copyAddr)}">${escapeHtml(tun.localHost === "0.0.0.0" ? `host.docker.internal:${tun.localPort}` : copy.tunnels.copyEndpoint)}</button>
                      <button class="btn btn-sm" data-tun-action="edit" data-id="${escapeHtml(tun.id)}" ${isRunning ? "disabled" : ""}>${escapeHtml(copy.common.edit)}</button>
                      <button class="btn btn-danger btn-sm" data-tun-action="delete" data-id="${escapeHtml(tun.id)}" ${isRunning ? "disabled" : ""}>${escapeHtml(copy.common.delete)}</button>
                    </div>
                  </div>
                `;
              })
              .join("")
      }
    </div>
  `;

  container.querySelector("#btn-new-tunnel")?.addEventListener("click", cb.onNewTunnel);

  const byId = new Map(state.tunnels.map((item) => [item.id, item]));

  container.querySelectorAll<HTMLButtonElement>("button[data-tun-action]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const tun = byId.get(btn.dataset.id ?? "");
      if (!tun) return;
      const action = btn.dataset.tunAction;
      if (action === "start") cb.onStartTunnel(tun);
      if (action === "stop") cb.onStopTunnel(tun);
      if (action === "edit") cb.onEditTunnel(tun);
      if (action === "delete") cb.onDeleteTunnel(tun);
    });
  });

  container.querySelectorAll<HTMLButtonElement>("button[data-tun-copy]").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const addr = btn.dataset.tunCopy ?? "";
      if (!addr) return;
      try {
        await navigator.clipboard.writeText(addr);
        const orig = btn.textContent ?? "";
        btn.textContent = t().tunnels.copied;
        setTimeout(() => {
          btn.textContent = orig;
        }, 1200);
      } catch {
        window.prompt("127.0.0.1:", addr);
      }
    });
  });
}
