import { t } from "../i18n";
import type { HostInstance, SSHKeySummary } from "../services/desktopApi";
import { escapeHtml } from "./dom";

export interface InstanceSidebarState {
  hosts: HostInstance[];
  keys: SSHKeySummary[];
  search: string;
  envFilter: string;
}

export interface InstanceSidebarCallbacks {
  onSearchChange: (search: string) => void;
  onEnvFilterChange: (env: string) => void;
  onNewHost: () => void;
  onConnectHost: (host: HostInstance) => void;
  onEditHost: (host: HostInstance) => void;
  onDeleteHost: (host: HostInstance) => void;
}

export function renderInstanceSidebar(
  container: HTMLElement,
  state: InstanceSidebarState,
  cb: InstanceSidebarCallbacks
): void {
  const copy = t();
  const keyNames = new Map(state.keys.map((k) => [k.id, k.name]));

  const q = state.search.toLowerCase();
  const filtered = state.hosts.filter((h) => {
    const matchesEnv = state.envFilter === "ALL" || h.environment === state.envFilter;
    const matchesSearch =
      !q ||
      h.alias.toLowerCase().includes(q) ||
      h.hostAddress.toLowerCase().includes(q) ||
      h.sshUsername.toLowerCase().includes(q);
    return matchesEnv && matchesSearch;
  });

  const envBadgeClass = (env: string) => {
    if (env === "Prod") return "badge badge-prod";
    if (env === "Staging") return "badge badge-staging";
    return "badge badge-dev";
  };

  container.innerHTML = `
    <div class="sidebar-header">
      <span>${escapeHtml(copy.sidebar.instancesTitle)}</span>
      <button id="btn-new-host" class="btn btn-primary btn-sm">${escapeHtml(copy.sidebar.newInstance)}</button>
    </div>
    <div class="sidebar-filters">
      <input id="host-search" class="input" placeholder="${escapeHtml(copy.sidebar.searchPlaceholder)}" value="${escapeHtml(state.search)}" />
      <div class="env-chips">
        ${["ALL", "Prod", "Staging", "Dev"]
          .map(
            (env) =>
              `<button class="chip ${state.envFilter === env ? "active" : ""}" data-env="${env}">${escapeHtml(env === "ALL" ? copy.sidebar.allEnvs : env)}</button>`
          )
          .join("")}
      </div>
    </div>
    <div class="host-list">
      ${
        filtered.length === 0
          ? `<div style="padding:16px;color:var(--text-secondary);text-align:center;">${escapeHtml(copy.sidebar.emptyState)}</div>`
          : filtered
              .map((h) => {
                const keyLabel = h.sshKeyId
                  ? (keyNames.get(h.sshKeyId) ?? copy.sidebar.noKeyAssigned)
                  : copy.sidebar.noKeyAssigned;
                return `
                  <div class="host-item" data-host-id="${escapeHtml(h.id)}">
                    <div class="host-item-top">
                      <span>${escapeHtml(h.alias)}</span>
                      <span class="${envBadgeClass(h.environment)}">${escapeHtml(h.environment)}</span>
                    </div>
                    <div class="host-item-sub">${escapeHtml(h.sshUsername)}@${escapeHtml(h.hostAddress)}:${h.port}</div>
                    <div style="margin-top:3px;font-size:11px;color:var(--text-secondary);">${escapeHtml(keyLabel)}</div>
                    <div class="host-item-actions">
                      <button class="btn btn-success btn-sm" data-action="connect" data-id="${escapeHtml(h.id)}">${escapeHtml(copy.sidebar.connect)}</button>
                      <button class="btn btn-sm" data-action="edit" data-id="${escapeHtml(h.id)}">${escapeHtml(copy.common.edit)}</button>
                      <button class="btn btn-danger btn-sm" data-action="delete" data-id="${escapeHtml(h.id)}">${escapeHtml(copy.common.delete)}</button>
                    </div>
                  </div>
                `;
              })
              .join("")
      }
    </div>
  `;

  container.querySelector("#btn-new-host")?.addEventListener("click", cb.onNewHost);

  const searchInput = container.querySelector<HTMLInputElement>("#host-search");
  searchInput?.addEventListener("input", () => {
    cb.onSearchChange(searchInput.value);
  });

  container.querySelectorAll<HTMLButtonElement>(".chip").forEach((chip) => {
    chip.addEventListener("click", () => {
      cb.onEnvFilterChange(chip.dataset.env ?? "ALL");
    });
  });

  const byId = new Map(state.hosts.map((h) => [h.id, h]));

  container.querySelectorAll<HTMLElement>(".host-item").forEach((item) => {
    item.addEventListener("dblclick", () => {
      const host = byId.get(item.dataset.hostId ?? "");
      if (host) cb.onConnectHost(host);
    });
  });

  container.querySelectorAll<HTMLButtonElement>("button[data-action]").forEach((btn) => {
    btn.addEventListener("click", (e) => {
      e.stopPropagation();
      const host = byId.get(btn.dataset.id ?? "");
      if (!host) return;
      const action = btn.dataset.action;
      if (action === "connect") cb.onConnectHost(host);
      if (action === "edit") cb.onEditHost(host);
      if (action === "delete") cb.onDeleteHost(host);
    });
  });
}
