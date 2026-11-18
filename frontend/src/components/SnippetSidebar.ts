import { t } from "../i18n";
import type { CommandSnippet, HostGroup, HostInstance } from "../services/desktopApi";
import { escapeHtml } from "./dom";

export interface SnippetSidebarState {
  snippets: CommandSnippet[];
  hosts: HostInstance[];
  groups: HostGroup[];
  search: string;
  hostFilter: string;
  groupFilter: string;
  activeHostId: string;
  activeSessionId: string;
  openSessionIds: string[];
}

export interface SnippetSidebarCallbacks {
  onSearchChange: (q: string) => void;
  onHostFilterChange: (hostId: string) => void;
  onGroupFilterChange: (groupId: string) => void;
  onNewSnippet: () => void;
  onEditSnippet: (snippet: CommandSnippet) => void;
  onDeleteSnippet: (snippet: CommandSnippet) => void;
  onRunSnippet: (snippet: CommandSnippet) => void;
  onBroadcastSnippet: (snippet: CommandSnippet) => void;
}

export function renderSnippetSidebarBody(
  container: HTMLElement,
  state: SnippetSidebarState,
  cb: SnippetSidebarCallbacks
): void {
  const copy = t();
  const hostById = new Map(state.hosts.map((h) => [h.id, h]));
  const groupNames = new Map(state.groups.map((g) => [g.id, g.name]));
  const query = state.search.trim().toLowerCase();
  const activeHost = state.activeHostId ? hostById.get(state.activeHostId) : undefined;
  const activeHostAlias = activeHost?.alias ?? "";

  const targetHostId =
    state.hostFilter === "ACTIVE"
      ? state.activeHostId
      : state.hostFilter !== "ALL" && state.hostFilter !== "GLOBAL"
        ? state.hostFilter
        : "";
  const targetHost = targetHostId ? hostById.get(targetHostId) : undefined;
  const effectiveRankHost = targetHost ?? activeHost;

  const effectiveGroupId = (snip: CommandSnippet): string => {
    if (snip.groupId) return snip.groupId;
    if (snip.hostId) return hostById.get(snip.hostId)?.groupId ?? "";
    return "";
  };

  const filtered = state.snippets.filter((snip) => {
    // Hierarchical instance applicability:
    // selecting an instance shows snippets bound to it, snippets shared with its group, and globals.
    if (targetHost) {
      const isDirectHost = snip.hostId === targetHost.id;
      const isHostGroupShared =
        targetHost.groupId !== "" && snip.hostId === "" && snip.groupId === targetHost.groupId;
      const isGlobal = snip.hostId === "" && snip.groupId === "";
      if (!isDirectHost && !isHostGroupShared && !isGlobal) return false;
    } else if (state.hostFilter === "GLOBAL") {
      if (snip.hostId !== "") return false;
    }

    const snipGroupId = effectiveGroupId(snip);
    if (state.groupFilter === "UNGROUPED") {
      if (snipGroupId !== "") return false;
    } else if (state.groupFilter !== "ALL") {
      if (snipGroupId !== state.groupFilter) return false;
    }

    if (!query) return true;
    const hName = snip.hostId ? (hostById.get(snip.hostId)?.alias ?? "") : "";
    const gName = snipGroupId ? (groupNames.get(snipGroupId) ?? "") : "";
    return (
      snip.name.toLowerCase().includes(query) ||
      snip.command.toLowerCase().includes(query) ||
      snip.notes.toLowerCase().includes(query) ||
      hName.toLowerCase().includes(query) ||
      gName.toLowerCase().includes(query)
    );
  });

  // Sort by specificity for the target or active instance: direct instance -> its group -> global -> others.
  const sorted = [...filtered].sort((a, b) => {
    const rank = (s: CommandSnippet): number => {
      if (effectiveRankHost) {
        if (s.hostId === effectiveRankHost.id) return 0;
        if (effectiveRankHost.groupId && s.hostId === "" && s.groupId === effectiveRankHost.groupId) {
          return 1;
        }
      }
      if (!s.hostId && !s.groupId) return 2;
      if (!s.hostId) return 3;
      return 4;
    };
    const rDiff = rank(a) - rank(b);
    if (rDiff !== 0) return rDiff;
    return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
  });

  const hasActiveSession = state.activeSessionId !== "";

  container.innerHTML = `
    <div class="sidebar-header">
      <span>${escapeHtml(copy.snippets.title)}</span>
      <button id="btn-new-snippet" class="btn btn-primary btn-sm">${escapeHtml(copy.snippets.newSnippet)}</button>
    </div>
    <div class="sidebar-filters">
      <input
        id="snippet-search"
        class="input"
        placeholder="${escapeHtml(copy.snippets.searchPlaceholder)}"
        value="${escapeHtml(state.search)}"
      />
      <select id="snippet-host-filter" class="select">
        <option value="ALL" ${state.hostFilter === "ALL" ? "selected" : ""}>${escapeHtml(copy.snippets.allHosts)}</option>
        ${
          state.activeHostId
            ? `<option value="ACTIVE" ${state.hostFilter === "ACTIVE" ? "selected" : ""}>${escapeHtml(copy.snippets.activeHost)}: ${escapeHtml(activeHostAlias)}</option>`
            : ""
        }
        <option value="GLOBAL" ${state.hostFilter === "GLOBAL" ? "selected" : ""}>${escapeHtml(copy.snippets.globalOnly)}</option>
        ${state.hosts
          .map(
            (h) =>
              `<option value="${escapeHtml(h.id)}" ${state.hostFilter === h.id ? "selected" : ""}>${escapeHtml(h.alias)} (${escapeHtml(h.hostAddress)})</option>`
          )
          .join("")}
      </select>
      <select id="snippet-group-filter" class="select">
        <option value="ALL" ${state.groupFilter === "ALL" ? "selected" : ""}>${escapeHtml(copy.snippets.allGroups)}</option>
        <option value="UNGROUPED" ${state.groupFilter === "UNGROUPED" ? "selected" : ""}>${escapeHtml(copy.snippets.ungroupedOnly)}</option>
        ${state.groups
          .map(
            (g) =>
              `<option value="${escapeHtml(g.id)}" ${state.groupFilter === g.id ? "selected" : ""}>${escapeHtml(g.name)}</option>`
          )
          .join("")}
      </select>
    </div>
    ${
      !hasActiveSession && state.snippets.length > 0
        ? `<div class="hint-box" style="margin:8px 10px 0;font-size:11px;">${escapeHtml(copy.snippets.openTerminalFirst)}</div>`
        : ""
    }
    <div class="host-list">
      ${
        state.snippets.length === 0
          ? `<div style="padding:16px;color:var(--text-secondary);text-align:center;line-height:1.45;">${escapeHtml(copy.snippets.emptyState)}</div>`
          : sorted.length === 0
            ? `<div style="padding:16px;color:var(--text-secondary);text-align:center;line-height:1.45;">${escapeHtml(copy.snippets.emptyFiltered)}</div>`
            : sorted
                .map((snip) => {
                  const hostLabel = snip.hostId ? (hostById.get(snip.hostId)?.alias ?? "") : "";
                  const groupId = effectiveGroupId(snip);
                  const groupLabel = groupId ? (groupNames.get(groupId) ?? "") : "";
                  const modeBadge = snip.autoExecute
                    ? `<span class="badge badge-dev">${escapeHtml(copy.snippets.autoExecBadge)}</span>`
                    : `<span class="badge badge-staging">${escapeHtml(copy.snippets.pasteOnlyBadge)}</span>`;
                  const runLabel = snip.autoExecute ? copy.snippets.run : copy.snippets.paste;

                  const metaParts: string[] = [];
                  if (hostLabel) {
                    const isHighlightedHost =
                      snip.hostId === (targetHostId || state.activeHostId);
                    metaParts.push(
                      `<span class="badge" style="background:${isHighlightedHost ? "rgba(56,189,248,0.22)" : "rgba(148,163,184,0.16)"};color:${isHighlightedHost ? "#7dd3fc" : "var(--text-primary)"};">${escapeHtml(hostLabel)}</span>`
                    );
                  }
                  if (groupLabel) {
                    metaParts.push(`<strong>${escapeHtml(groupLabel)}</strong>`);
                  } else if (!hostLabel) {
                    metaParts.push(
                      `<span class="badge" style="background:rgba(148,163,184,0.16);color:var(--text-secondary);">${escapeHtml(copy.snippets.globalBadge)}</span>`
                    );
                  }
                  if (snip.notes) {
                    metaParts.push(escapeHtml(snip.notes));
                  }

                  return `
                    <div class="host-item" data-snip-id="${escapeHtml(snip.id)}">
                      <div class="host-item-top">
                        <span>${escapeHtml(snip.name)}</span>
                        ${modeBadge}
                      </div>
                      <div style="margin-top:4px;font-size:11px;color:var(--text-secondary);display:flex;align-items:center;gap:6px;flex-wrap:wrap;">
                        ${metaParts.join(" · ")}
                      </div>
                      <pre class="snippet-preview">${escapeHtml(snip.command)}</pre>
                      <div class="host-item-actions" style="flex-wrap:wrap;">
                        <button
                          class="btn btn-success btn-sm"
                          data-snip-action="run"
                          data-id="${escapeHtml(snip.id)}"
                          ${hasActiveSession ? "" : "disabled"}
                          title="${hasActiveSession ? "" : escapeHtml(copy.snippets.openTerminalFirst)}"
                        >${escapeHtml(runLabel)}</button>
                        ${
                          state.openSessionIds.length >= 2
                            ? `<button class="btn btn-primary btn-sm" data-snip-action="broadcast" data-id="${escapeHtml(snip.id)}">${escapeHtml(copy.snippets.broadcastAll)} (${state.openSessionIds.length})</button>`
                            : ""
                        }
                        <button class="btn btn-sm" data-snip-action="edit" data-id="${escapeHtml(snip.id)}">${escapeHtml(copy.common.edit)}</button>
                        <button class="btn btn-danger btn-sm" data-snip-action="delete" data-id="${escapeHtml(snip.id)}">${escapeHtml(copy.common.delete)}</button>
                      </div>
                    </div>
                  `;
                })
                .join("")
      }
    </div>
  `;

  container.querySelector("#btn-new-snippet")?.addEventListener("click", cb.onNewSnippet);

  container.querySelector<HTMLInputElement>("#snippet-search")?.addEventListener("input", (e) => {
    cb.onSearchChange((e.target as HTMLInputElement).value);
  });

  container
    .querySelector<HTMLSelectElement>("#snippet-host-filter")
    ?.addEventListener("change", (e) => {
      cb.onHostFilterChange((e.target as HTMLSelectElement).value);
    });

  container
    .querySelector<HTMLSelectElement>("#snippet-group-filter")
    ?.addEventListener("change", (e) => {
      cb.onGroupFilterChange((e.target as HTMLSelectElement).value);
    });

  const byId = new Map(state.snippets.map((item) => [item.id, item]));
  container.querySelectorAll<HTMLButtonElement>("button[data-snip-action]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const snip = byId.get(btn.dataset.id ?? "");
      if (!snip) return;
      const action = btn.dataset.snipAction;
      if (action === "run") cb.onRunSnippet(snip);
      if (action === "broadcast") cb.onBroadcastSnippet(snip);
      if (action === "edit") cb.onEditSnippet(snip);
      if (action === "delete") cb.onDeleteSnippet(snip);
    });
  });
}
