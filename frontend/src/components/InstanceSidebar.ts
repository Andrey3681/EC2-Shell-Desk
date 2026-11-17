import { t } from "../i18n";
import type {
  HostGroup,
  HostInstance,
  SSHKeySummary,
} from "../services/desktopApi";
import { escapeHtml } from "./dom";

export interface InstanceSidebarState {
  hosts: HostInstance[];
  groups: HostGroup[];
  keys: SSHKeySummary[];
  search: string;
  envFilter: string;
  collapsedGroups: Set<string>;
}

export interface InstanceSidebarCallbacks {
  onSearchChange: (search: string) => void;
  onEnvFilterChange: (env: string) => void;
  onToggleGroup: (groupKey: string) => void;
  onNewGroup: () => void;
  onRenameGroup: (group: HostGroup) => void;
  onDeleteGroup: (group: HostGroup) => void;
  onNewHost: (defaultGroupId?: string) => void;
  onConnectHost: (host: HostInstance) => void;
  onEditHost: (host: HostInstance) => void;
  onDeleteHost: (host: HostInstance) => void;
}

const UNGROUPED_KEY = "__ungrouped__";

export function renderInstanceSidebar(
  container: HTMLElement,
  state: InstanceSidebarState,
  cb: InstanceSidebarCallbacks
): void {
  const copy = t();
  const keyNames = new Map(state.keys.map((k) => [k.id, k.name]));
  const groupNames = new Map(state.groups.map((g) => [g.id, g.name]));
  const groupById = new Map(state.groups.map((g) => [g.id, g]));
  const hostById = new Map(state.hosts.map((h) => [h.id, h]));

  const q = state.search.trim().toLowerCase();
  const isFiltering = q !== "" || state.envFilter !== "ALL";

  const matchesHost = (h: HostInstance): boolean => {
    const matchesEnv = state.envFilter === "ALL" || h.environment === state.envFilter;
    const grpName = h.groupId ? (groupNames.get(h.groupId) ?? "") : "";
    const matchesSearch =
      !q ||
      h.alias.toLowerCase().includes(q) ||
      h.hostAddress.toLowerCase().includes(q) ||
      h.sshUsername.toLowerCase().includes(q) ||
      grpName.toLowerCase().includes(q);
    return matchesEnv && matchesSearch;
  };

  const envBadgeClass = (env: string) => {
    if (env === "Prod") return "badge badge-prod";
    if (env === "Staging") return "badge badge-staging";
    return "badge badge-dev";
  };

  const renderHostCard = (h: HostInstance): string => {
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
  };

  const buildListHtml = (): string => {
    if (state.hosts.length === 0 && state.groups.length === 0) {
      return `<div style="padding:16px;color:var(--text-secondary);text-align:center;">${escapeHtml(copy.sidebar.emptyState)}</div>`;
    }

    const sections: string[] = [];

    for (const grp of state.groups) {
      const members = state.hosts.filter((h) => h.groupId === grp.id && matchesHost(h));
      if (isFiltering && members.length === 0) {
        continue;
      }
      const isCollapsed = !isFiltering && state.collapsedGroups.has(grp.id);
      sections.push(`
        <div class="group-section">
          <div class="group-header" data-group-toggle="${escapeHtml(grp.id)}">
            <div class="group-title">
              <span>${isCollapsed ? "&#9656;" : "&#9662;"}</span>
              <span class="group-title-text">${escapeHtml(grp.name)}</span>
              <span class="group-count">${members.length}</span>
            </div>
            <div class="group-actions">
              <button class="btn btn-sm" data-grp-action="add-host" data-gid="${escapeHtml(grp.id)}" title="${escapeHtml(copy.sidebar.newInstance)}">+</button>
              <button class="btn btn-sm" data-grp-action="rename" data-gid="${escapeHtml(grp.id)}" title="${escapeHtml(copy.common.edit)}">&#9998;</button>
              <button class="btn btn-danger btn-sm" data-grp-action="delete" data-gid="${escapeHtml(grp.id)}" title="${escapeHtml(copy.common.delete)}">&times;</button>
            </div>
          </div>
          ${
            isCollapsed
              ? ""
              : `<div class="group-items">
                  ${
                    members.length === 0
                      ? `<div class="group-empty">${escapeHtml(copy.sidebar.emptyGroup)}</div>`
                      : members.map(renderHostCard).join("")
                  }
                </div>`
          }
        </div>
      `);
    }

    const ungrouped = state.hosts.filter(
      (h) => (!h.groupId || !groupById.has(h.groupId)) && matchesHost(h)
    );
    if (ungrouped.length > 0) {
      if (state.groups.length === 0) {
        // When user has no groups yet, render cards directly cleanly as before.
        sections.push(ungrouped.map(renderHostCard).join(""));
      } else {
        const isCollapsed = !isFiltering && state.collapsedGroups.has(UNGROUPED_KEY);
        sections.push(`
          <div class="group-section">
            <div class="group-header" data-group-toggle="${UNGROUPED_KEY}">
              <div class="group-title">
                <span>${isCollapsed ? "&#9656;" : "&#9662;"}</span>
                <span class="group-title-text">${escapeHtml(copy.sidebar.ungrouped)}</span>
                <span class="group-count">${ungrouped.length}</span>
              </div>
              <div class="group-actions">
                <button class="btn btn-sm" data-grp-action="add-host" data-gid="" title="${escapeHtml(copy.sidebar.newInstance)}">+</button>
              </div>
            </div>
            ${isCollapsed ? "" : `<div class="group-items">${ungrouped.map(renderHostCard).join("")}</div>`}
          </div>
        `);
      }
    }

    if (sections.length === 0) {
      return `<div style="padding:16px;color:var(--text-secondary);text-align:center;">${escapeHtml(copy.sidebar.emptyFiltered)}</div>`;
    }

    return sections.join("");
  };

  container.innerHTML = `
    <div class="sidebar-header">
      <span>${escapeHtml(copy.sidebar.instancesTitle)}</span>
      <div style="display:flex;gap:4px;">
        <button id="btn-new-group" class="btn btn-sm">${escapeHtml(copy.sidebar.newGroup)}</button>
        <button id="btn-new-host" class="btn btn-primary btn-sm">${escapeHtml(copy.sidebar.newInstance)}</button>
      </div>
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
    <div id="host-list" class="host-list">
      ${buildListHtml()}
    </div>
  `;

  container.querySelector("#btn-new-group")?.addEventListener("click", cb.onNewGroup);
  container.querySelector("#btn-new-host")?.addEventListener("click", () => cb.onNewHost(""));

  const searchInput = container.querySelector<HTMLInputElement>("#host-search");
  searchInput?.addEventListener("input", () => {
    cb.onSearchChange(searchInput.value);
  });

  container.querySelectorAll<HTMLButtonElement>(".chip").forEach((chip) => {
    chip.addEventListener("click", () => {
      cb.onEnvFilterChange(chip.dataset.env ?? "ALL");
    });
  });

  container.querySelectorAll<HTMLElement>("[data-group-toggle]").forEach((hdr) => {
    hdr.addEventListener("click", () => {
      const key = hdr.dataset.groupToggle ?? "";
      if (key) cb.onToggleGroup(key);
    });
  });

  container.querySelectorAll<HTMLButtonElement>("button[data-grp-action]").forEach((btn) => {
    btn.addEventListener("click", (e) => {
      e.stopPropagation();
      const action = btn.dataset.grpAction;
      const gid = btn.dataset.gid ?? "";
      if (action === "add-host") {
        cb.onNewHost(gid);
        return;
      }
      const grp = groupById.get(gid);
      if (!grp) return;
      if (action === "rename") cb.onRenameGroup(grp);
      if (action === "delete") cb.onDeleteGroup(grp);
    });
  });

  container.querySelectorAll<HTMLElement>(".host-item").forEach((item) => {
    item.addEventListener("dblclick", () => {
      const host = hostById.get(item.dataset.hostId ?? "");
      if (host) cb.onConnectHost(host);
    });
  });

  container.querySelectorAll<HTMLButtonElement>("button[data-action]").forEach((btn) => {
    btn.addEventListener("click", (e) => {
      e.stopPropagation();
      const host = hostById.get(btn.dataset.id ?? "");
      if (!host) return;
      const action = btn.dataset.action;
      if (action === "connect") cb.onConnectHost(host);
      if (action === "edit") cb.onEditHost(host);
      if (action === "delete") cb.onDeleteHost(host);
    });
  });
}
