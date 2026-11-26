import "./styles/app.css";
import { renderAuthView } from "./components/AuthView";
import { escapeHtml, extractErrorMessage } from "./components/dom";
import { openGroupModal } from "./components/GroupModal";
import { openHostModal } from "./components/HostModal";
import { renderInstanceSidebar } from "./components/InstanceSidebar";
import { openKeyVaultModal } from "./components/KeyVaultModal";
import { openServiceModal } from "./components/ServiceModal";
import { renderServiceSidebarBody } from "./components/ServiceSidebar";
import { openSnippetModal } from "./components/SnippetModal";
import { renderSnippetSidebarBody } from "./components/SnippetSidebar";
import { TerminalTabsManager } from "./components/TerminalTabs";
import { openTofuModal } from "./components/TofuModal";
import { openTunnelModal } from "./components/TunnelModal";
import { renderTunnelSidebarBody } from "./components/TunnelSidebar";
import { XftpDualPane } from "./components/XftpDualPane";
import { getLocale, setLocale, t, type Locale } from "./i18n";
import {
  desktopApi,
  type CommandSnippet,
  type HostGroup,
  type HostInstance,
  type PrivateService,
  type SSHKeySummary,
  type SSHTunnel,
  type ServiceRuntimeStatus,
  type TunnelRuntimeStatus,
  type UserProfile,
} from "./services/desktopApi";

const rootEl = document.getElementById("app") as HTMLElement;

let activeUser: UserProfile | null = null;
let sidebarTab: "hosts" | "tunnels" | "snippets" | "services" = "hosts";
let hosts: HostInstance[] = [];
let groups: HostGroup[] = [];
let keys: SSHKeySummary[] = [];
let tunnels: SSHTunnel[] = [];
let snippets: CommandSnippet[] = [];
let servicesList: PrivateService[] = [];
const tunnelStatuses = new Map<string, TunnelRuntimeStatus>();
const serviceStatuses = new Map<string, ServiceRuntimeStatus>();
let searchFilter = "";
let envFilter = "ALL";
let snippetSearch = "";
let snippetHostFilter = "ALL";
let snippetGroupFilter = "ALL";
let serviceSearch = "";
let serviceHostFilter = "ALL";
const collapsedGroups = new Set<string>();
let terminalMgr: TerminalTabsManager | null = null;
let xftpPane: XftpDualPane | null = null;

desktopApi.terminal.onTOFUPrompt((challenge) => {
  openTofuModal(challenge);
});

desktopApi.tunnels.onStatus((status) => {
  if (status.state === "stopped") {
    tunnelStatuses.delete(status.tunnelId);
  } else {
    tunnelStatuses.set(status.tunnelId, status);
  }
  const sidebarBodyEl = rootEl.querySelector<HTMLElement>("#sidebar-body");
  if (sidebarBodyEl && sidebarTab === "tunnels") {
    drawSidebarContent(sidebarBodyEl);
  }
});

desktopApi.services.onStatus((status) => {
  if (status.state === "stopped") {
    serviceStatuses.delete(status.serviceId);
  } else {
    serviceStatuses.set(status.serviceId, status);
  }
  const sidebarBodyEl = rootEl.querySelector<HTMLElement>("#sidebar-body");
  if (sidebarBodyEl && sidebarTab === "services") {
    drawSidebarContent(sidebarBodyEl);
  }
});

async function bootstrapApp(): Promise<void> {
  try {
    const status = await desktopApi.auth.status();
    if (status.unlocked && status.user) {
      activeUser = status.user;
      await mountWorkspace();
      return;
    }
    mountAuth(status.hasUsers);
  } catch (err) {
    rootEl.innerHTML = `<div class="auth-wrapper"><div class="error-banner">${escapeHtml(extractErrorMessage(err))}</div></div>`;
  }
}

function mountAuth(hasUsers: boolean): void {
  terminalMgr?.disposeAll();
  terminalMgr = null;
  xftpPane?.dispose();
  xftpPane = null;
  activeUser = null;
  tunnelStatuses.clear();
  serviceStatuses.clear();
  collapsedGroups.clear();
  snippets = [];
  servicesList = [];
  snippetSearch = "";
  snippetHostFilter = "ALL";
  snippetGroupFilter = "ALL";
  serviceSearch = "";
  serviceHostFilter = "ALL";

  renderAuthView(rootEl, {
    hasUsers,
    onAuthenticated: (user) => {
      activeUser = user;
      void mountWorkspace();
    },
    onLocaleChanged: () => {
      // AuthView redraws itself on locale change.
    },
  });
}

async function refreshInventory(): Promise<void> {
  try {
    const [
      loadedHosts,
      loadedGroups,
      loadedKeys,
      loadedTunnels,
      loadedStatuses,
      loadedSnippets,
      loadedServices,
      loadedServiceStatuses,
    ] = await Promise.all([
      desktopApi.hosts.list(),
      desktopApi.groups.list(),
      desktopApi.keys.list(),
      desktopApi.tunnels.list(),
      desktopApi.tunnels.listStatuses(),
      desktopApi.snippets.list(),
      desktopApi.services.list(),
      desktopApi.services.listStatuses(),
    ]);
    hosts = loadedHosts ?? [];
    groups = loadedGroups ?? [];
    keys = loadedKeys ?? [];
    tunnels = loadedTunnels ?? [];
    snippets = loadedSnippets ?? [];
    servicesList = loadedServices ?? [];
    tunnelStatuses.clear();
    for (const st of loadedStatuses ?? []) {
      tunnelStatuses.set(st.tunnelId, st);
    }
    serviceStatuses.clear();
    for (const st of loadedServiceStatuses ?? []) {
      serviceStatuses.set(st.serviceId, st);
    }
  } catch (err) {
    hosts = hosts ?? [];
    groups = groups ?? [];
    keys = keys ?? [];
    tunnels = tunnels ?? [];
    snippets = snippets ?? [];
    servicesList = servicesList ?? [];
    window.alert(extractErrorMessage(err));
  }
  drawSidebarShell();
}

function drawSidebarShell(): void {
  const sidebarEl = rootEl.querySelector<HTMLElement>("#sidebar");
  if (!sidebarEl) return;
  const copy = t();
  const activeCount = Array.from(tunnelStatuses.values()).filter(
    (s) => s.state === "active"
  ).length;
  const activeSvcCount = Array.from(serviceStatuses.values()).filter(
    (s) => s.state === "active"
  ).length;

  sidebarEl.innerHTML = `
    <div class="sidebar-tabs">
      <button id="stab-hosts" class="sidebar-tab-btn ${sidebarTab === "hosts" ? "active" : ""}">
        ${escapeHtml(copy.sidebar.tabInstances)} (${hosts.length})
      </button>
      <button id="stab-tunnels" class="sidebar-tab-btn ${sidebarTab === "tunnels" ? "active" : ""}">
        ${escapeHtml(copy.sidebar.tabTunnels)} (${activeCount > 0 ? `${activeCount}/` : ""}${tunnels.length})
      </button>
      <button id="stab-snippets" class="sidebar-tab-btn ${sidebarTab === "snippets" ? "active" : ""}">
        ${escapeHtml(copy.sidebar.tabSnippets)} (${snippets.length})
      </button>
      <button id="stab-services" class="sidebar-tab-btn ${sidebarTab === "services" ? "active" : ""}">
        ${escapeHtml(copy.sidebar.tabServices)} (${activeSvcCount > 0 ? `${activeSvcCount}/` : ""}${servicesList.length})
      </button>
    </div>
    <div id="sidebar-body" style="flex:1;min-height:0;display:flex;flex-direction:column;"></div>
  `;

  sidebarEl.querySelector("#stab-hosts")?.addEventListener("click", () => {
    sidebarTab = "hosts";
    drawSidebarShell();
  });
  sidebarEl.querySelector("#stab-tunnels")?.addEventListener("click", () => {
    sidebarTab = "tunnels";
    drawSidebarShell();
  });
  sidebarEl.querySelector("#stab-snippets")?.addEventListener("click", () => {
    sidebarTab = "snippets";
    drawSidebarShell();
  });
  sidebarEl.querySelector("#stab-services")?.addEventListener("click", () => {
    sidebarTab = "services";
    drawSidebarShell();
  });

  const bodyEl = sidebarEl.querySelector<HTMLElement>("#sidebar-body");
  if (bodyEl) {
    drawSidebarContent(bodyEl);
  }
}

function drawSidebarContent(bodyEl: HTMLElement): void {
  if (sidebarTab === "tunnels") {
    renderTunnelSidebarBody(
      bodyEl,
      { tunnels, hosts, statuses: tunnelStatuses },
      {
        onNewTunnel: () => {
          openTunnelModal(null, hosts, () => {
            void refreshInventory();
          });
        },
        onStartTunnel: (tun) => {
          void startTunnel(tun);
        },
        onStopTunnel: async (tun) => {
          try {
            await desktopApi.tunnels.stop(tun.id);
            await refreshInventory();
          } catch (err) {
            window.alert(extractErrorMessage(err));
          }
        },
        onEditTunnel: (tun) => {
          openTunnelModal(tun, hosts, () => {
            void refreshInventory();
          });
        },
        onDeleteTunnel: async (tun) => {
          if (!window.confirm(t().common.confirmDelete)) return;
          try {
            await desktopApi.tunnels.delete(tun.id);
            await refreshInventory();
          } catch (err) {
            window.alert(extractErrorMessage(err));
          }
        },
      }
    );
    return;
  }

  if (sidebarTab === "snippets") {
    const prevSnipSearch = bodyEl.querySelector<HTMLInputElement>("#snippet-search");
    const hadFocus = document.activeElement === prevSnipSearch;
    const cursorPos = prevSnipSearch?.selectionStart ?? null;
    const activeHostId = terminalMgr?.getActiveHostId() ?? "";
    if (snippetHostFilter === "ACTIVE" && !activeHostId) {
      snippetHostFilter = "ALL";
    } else if (
      snippetHostFilter !== "ALL" &&
      snippetHostFilter !== "ACTIVE" &&
      snippetHostFilter !== "GLOBAL" &&
      !hosts.some((h) => h.id === snippetHostFilter)
    ) {
      snippetHostFilter = "ALL";
    }

    renderSnippetSidebarBody(
      bodyEl,
      {
        snippets,
        hosts,
        groups,
        search: snippetSearch,
        hostFilter: snippetHostFilter,
        groupFilter: snippetGroupFilter,
        activeHostId,
        activeSessionId: terminalMgr?.getActiveSessionId() ?? "",
        openSessionIds: terminalMgr?.getOpenSessionIds() ?? [],
      },
      {
        onSearchChange: (q) => {
          snippetSearch = q;
          drawSidebarContent(bodyEl);
        },
        onHostFilterChange: (hostId) => {
          snippetHostFilter = hostId;
          drawSidebarContent(bodyEl);
        },
        onGroupFilterChange: (groupId) => {
          snippetGroupFilter = groupId;
          drawSidebarContent(bodyEl);
        },
        onNewSnippet: () => {
          const defaultHostId =
            snippetHostFilter === "ACTIVE"
              ? activeHostId
              : snippetHostFilter !== "ALL" && snippetHostFilter !== "GLOBAL"
                ? snippetHostFilter
                : activeHostId;
          openSnippetModal(
            null,
            hosts,
            groups,
            () => {
              void refreshInventory();
            },
            defaultHostId
          );
        },
        onEditSnippet: (snip) => {
          openSnippetModal(snip, hosts, groups, () => {
            void refreshInventory();
          });
        },
        onDeleteSnippet: async (snip) => {
          if (!window.confirm(t().common.confirmDelete)) return;
          try {
            await desktopApi.snippets.delete(snip.id);
            await refreshInventory();
          } catch (err) {
            window.alert(extractErrorMessage(err));
          }
        },
        onRunSnippet: async (snip) => {
          const sid = terminalMgr?.getActiveSessionId() ?? "";
          if (!sid) {
            window.alert(t().snippets.openTerminalFirst);
            return;
          }
          try {
            await desktopApi.snippets.run(sid, snip.id);
            terminalMgr?.focusActive();
          } catch (err) {
            window.alert(extractErrorMessage(err));
          }
        },
        onBroadcastSnippet: async (snip) => {
          const sids = terminalMgr?.getOpenSessionIds() ?? [];
          if (sids.length === 0) {
            window.alert(t().snippets.openTerminalFirst);
            return;
          }
          try {
            await desktopApi.snippets.broadcast(sids, snip.id);
            terminalMgr?.focusActive();
          } catch (err) {
            window.alert(extractErrorMessage(err));
          }
        },
      }
    );

    if (hadFocus) {
      const nextSnipSearch = bodyEl.querySelector<HTMLInputElement>("#snippet-search");
      if (nextSnipSearch) {
        nextSnipSearch.focus();
        if (cursorPos !== null) {
          nextSnipSearch.setSelectionRange(cursorPos, cursorPos);
        }
      }
    }
    return;
  }

  if (sidebarTab === "services") {
    const prevSvcSearch = bodyEl.querySelector<HTMLInputElement>("#service-search");
    const hadFocus = document.activeElement === prevSvcSearch;
    const cursorPos = prevSvcSearch?.selectionStart ?? null;
    const activeHostId = terminalMgr?.getActiveHostId() ?? "";
    if (serviceHostFilter === "ACTIVE" && !activeHostId) {
      serviceHostFilter = "ALL";
    } else if (
      serviceHostFilter !== "ALL" &&
      serviceHostFilter !== "ACTIVE" &&
      !hosts.some((h) => h.id === serviceHostFilter)
    ) {
      serviceHostFilter = "ALL";
    }

    renderServiceSidebarBody(
      bodyEl,
      {
        services: servicesList,
        hosts,
        statuses: serviceStatuses,
        search: serviceSearch,
        hostFilter: serviceHostFilter,
        activeHostId,
      },
      {
        onSearchChange: (q) => {
          serviceSearch = q;
          drawSidebarContent(bodyEl);
        },
        onHostFilterChange: (hostId) => {
          serviceHostFilter = hostId;
          drawSidebarContent(bodyEl);
        },
        onNewService: () => {
          const defaultHostId =
            serviceHostFilter === "ACTIVE"
              ? activeHostId
              : serviceHostFilter !== "ALL"
                ? serviceHostFilter
                : activeHostId;
          openServiceModal(
            null,
            hosts,
            () => {
              void refreshInventory();
            },
            defaultHostId
          );
        },
        onOpenService: async (svc) => {
          await openServiceAction(svc);
        },
        onStartService: async (svc) => {
          await startServiceAction(svc);
        },
        onStopService: async (svc) => {
          try {
            await desktopApi.services.stop(svc.id);
            await refreshInventory();
          } catch (err) {
            window.alert(extractErrorMessage(err));
          }
        },
        onProbeHealth: async (svc) => {
          try {
            const st = await desktopApi.services.probe(svc.id);
            serviceStatuses.set(svc.id, st);
            drawSidebarContent(bodyEl);
          } catch (err) {
            window.alert(extractErrorMessage(err));
          }
        },
        onEditService: (svc) => {
          openServiceModal(svc, hosts, () => {
            void refreshInventory();
          });
        },
        onDeleteService: async (svc) => {
          if (!window.confirm(t().common.confirmDelete)) return;
          try {
            await desktopApi.services.delete(svc.id);
            await refreshInventory();
          } catch (err) {
            window.alert(extractErrorMessage(err));
          }
        },
      }
    );

    if (hadFocus) {
      const nextSvcSearch = bodyEl.querySelector<HTMLInputElement>("#service-search");
      if (nextSvcSearch) {
        nextSvcSearch.focus();
        if (cursorPos !== null) {
          nextSvcSearch.setSelectionRange(cursorPos, cursorPos);
        }
      }
    }
    return;
  }

  const prevSearch = bodyEl.querySelector<HTMLInputElement>("#host-search");
  const hadFocus = document.activeElement === prevSearch;
  const cursorPos = prevSearch?.selectionStart ?? null;
  const snippetCountsByHost = new Map<string, number>();
  for (const s of snippets) {
    if (s.hostId) {
      snippetCountsByHost.set(s.hostId, (snippetCountsByHost.get(s.hostId) ?? 0) + 1);
    }
  }
  const serviceCountsByHost = new Map<string, number>();
  for (const s of servicesList) {
    if (s.hostId) {
      serviceCountsByHost.set(s.hostId, (serviceCountsByHost.get(s.hostId) ?? 0) + 1);
    }
  }

  renderInstanceSidebar(
    bodyEl,
    {
      hosts,
      groups,
      keys,
      snippetCountsByHost,
      serviceCountsByHost,
      search: searchFilter,
      envFilter,
      collapsedGroups,
    },
    {
      onSearchChange: (q) => {
        searchFilter = q;
        drawSidebarContent(bodyEl);
      },
      onEnvFilterChange: (env) => {
        envFilter = env;
        drawSidebarContent(bodyEl);
      },
      onToggleGroup: (groupKey) => {
        if (collapsedGroups.has(groupKey)) {
          collapsedGroups.delete(groupKey);
        } else {
          collapsedGroups.add(groupKey);
        }
        drawSidebarContent(bodyEl);
      },
      onNewGroup: () => {
        openGroupModal(null, () => {
          void refreshInventory();
        });
      },
      onRenameGroup: (group) => {
        openGroupModal(group, () => {
          void refreshInventory();
        });
      },
      onDeleteGroup: async (group) => {
        if (!window.confirm(t().sidebar.deleteGroupConfirm)) return;
        try {
          await desktopApi.groups.delete(group.id);
          await refreshInventory();
        } catch (err) {
          window.alert(extractErrorMessage(err));
        }
      },
      onNewHost: (defaultGroupId = "") => {
        openHostModal(
          null,
          keys,
          groups,
          () => {
            void refreshInventory();
          },
          defaultGroupId
        );
      },
      onConnectHost: (host) => {
        void connectHost(host);
      },
      onOpenHostSnippets: (host) => {
        snippetHostFilter = host.id;
        snippetGroupFilter = "ALL";
        snippetSearch = "";
        sidebarTab = "snippets";
        drawSidebarShell();
      },
      onOpenHostServices: (host) => {
        serviceHostFilter = host.id;
        serviceSearch = "";
        sidebarTab = "services";
        drawSidebarShell();
      },
      onEditHost: (host) => {
        openHostModal(host, keys, groups, () => {
          void refreshInventory();
        });
      },
      onDeleteHost: async (host) => {
        if (!window.confirm(t().common.confirmDelete)) return;
        try {
          await desktopApi.hosts.delete(host.id);
          await refreshInventory();
        } catch (err) {
          window.alert(extractErrorMessage(err));
        }
      },
    }
  );

  if (hadFocus) {
    const nextSearch = bodyEl.querySelector<HTMLInputElement>("#host-search");
    if (nextSearch) {
      nextSearch.focus();
      if (cursorPos !== null) {
        nextSearch.setSelectionRange(cursorPos, cursorPos);
      }
    }
  }
}

async function startTunnel(tun: SSHTunnel): Promise<void> {
  const bastion = hosts.find((h) => h.id === tun.hostId);
  let passwordFallback = "";
  if (bastion && !bastion.sshKeyId) {
    passwordFallback = window.prompt(t().terminal.passwordPrompt) ?? "";
  }
  try {
    const status = await desktopApi.tunnels.start(tun.id, passwordFallback);
    tunnelStatuses.set(tun.id, status);
    drawSidebarShell();
  } catch (err) {
    window.alert(extractErrorMessage(err));
    drawSidebarShell();
  }
}

async function startServiceAction(svc: PrivateService): Promise<void> {
  const host = hosts.find((h) => h.id === svc.hostId);
  let passwordFallback = "";
  if (host && !host.sshKeyId) {
    passwordFallback = window.prompt(t().terminal.passwordPrompt) ?? "";
  }
  try {
    const status = await desktopApi.services.start(svc.id, passwordFallback);
    serviceStatuses.set(svc.id, status);
    drawSidebarShell();
  } catch (err) {
    window.alert(extractErrorMessage(err));
    drawSidebarShell();
  }
}

async function openServiceAction(svc: PrivateService): Promise<void> {
  const host = hosts.find((h) => h.id === svc.hostId);
  let passwordFallback = "";
  if (host && !host.sshKeyId) {
    passwordFallback = window.prompt(t().terminal.passwordPrompt) ?? "";
  }
  try {
    const status = await desktopApi.services.open(svc.id, passwordFallback);
    serviceStatuses.set(svc.id, status);
    drawSidebarShell();
  } catch (err) {
    window.alert(extractErrorMessage(err));
    drawSidebarShell();
  }
}

async function connectHost(host: HostInstance): Promise<void> {
  if (!terminalMgr) return;
  let passwordFallback = "";
  if (!host.sshKeyId) {
    passwordFallback = window.prompt(t().terminal.passwordPrompt) ?? "";
  }
  const dims = terminalMgr.estimateDimensions();
  try {
    const session = await desktopApi.terminal.openSession(
      host.id,
      dims.cols,
      dims.rows,
      passwordFallback
    );
    terminalMgr.addSession(session);
  } catch (err) {
    window.alert(extractErrorMessage(err));
  }
}

async function mountWorkspace(): Promise<void> {
  const copy = t();
  const locale = getLocale();

  rootEl.innerHTML = `
    <div class="workspace">
      <header class="topbar">
        <div class="topbar-brand">
          <span>${escapeHtml(copy.appName)}</span>
          <span class="user-pill">${escapeHtml(activeUser?.username ?? "")}</span>
        </div>
        <div class="topbar-actions">
          <button id="btn-vault" class="btn btn-sm">${escapeHtml(copy.vaultModal.title)}</button>
          <select id="ws-locale" class="select" style="width:68px;padding:3px 6px;">
            <option value="es" ${locale === "es" ? "selected" : ""}>ES</option>
            <option value="en" ${locale === "en" ? "selected" : ""}>EN</option>
          </select>
          <button id="btn-lock" class="btn btn-danger btn-sm">${escapeHtml(copy.auth.lockSession)}</button>
        </div>
      </header>
      <div class="main-split">
        <aside id="sidebar" class="sidebar"></aside>
        <section class="center-pane">
          <div id="tabs-bar" class="tabs-bar"></div>
          <div id="terminal-stage" class="terminal-stage">
            <div class="welcome-pane">
              <h2 style="margin:0 0 8px;color:var(--text-primary);">${escapeHtml(copy.terminal.welcomeTitle)}</h2>
              <p style="max-width:480px;margin:0;">${escapeHtml(copy.terminal.welcomeDesc)}</p>
            </div>
          </div>
          <div id="xftp-panel" class="xftp-panel" style="display:none;"></div>
        </section>
      </div>
    </div>
  `;

  const tabsBar = rootEl.querySelector<HTMLElement>("#tabs-bar")!;
  const stageEl = rootEl.querySelector<HTMLElement>("#terminal-stage")!;
  const xftpEl = rootEl.querySelector<HTMLElement>("#xftp-panel")!;

  const resolveSessionContextLabel = (sessionId: string): string => {
    if (!sessionId) return "";
    const hostId = terminalMgr?.getActiveHostId() ?? "";
    const host = hosts.find((h) => h.id === hostId);
    if (host) {
      return `${host.alias} (${host.hostAddress})`;
    }
    return sessionId;
  };

  xftpPane = new XftpDualPane(xftpEl, () => {
    xftpEl.style.display = "none";
  });
  terminalMgr = new TerminalTabsManager(
    tabsBar,
    stageEl,
    (sessionId) => {
      if (xftpEl.style.display !== "none") {
        void xftpPane?.setSession(sessionId, resolveSessionContextLabel(sessionId));
      }
      if (sidebarTab === "snippets") {
        const bodyEl = rootEl.querySelector<HTMLElement>("#sidebar-body");
        if (bodyEl) {
          drawSidebarContent(bodyEl);
        }
      }
    },
    (visible) => {
      xftpEl.style.display = visible ? "flex" : "none";
      if (visible) {
        const sid = terminalMgr?.getActiveSessionId() ?? "";
        void xftpPane?.setSession(sid, resolveSessionContextLabel(sid));
      }
    }
  );

  rootEl.querySelector("#btn-vault")?.addEventListener("click", () => {
    openKeyVaultModal(() => {
      void refreshInventory();
    });
  });

  rootEl.querySelector<HTMLSelectElement>("#ws-locale")?.addEventListener("change", (e) => {
    setLocale((e.target as HTMLSelectElement).value as Locale);
    void mountWorkspace();
  });

  rootEl.querySelector("#btn-lock")?.addEventListener("click", async () => {
    await desktopApi.auth.lock();
    mountAuth(true);
  });

  await refreshInventory();
}

void bootstrapApp();
