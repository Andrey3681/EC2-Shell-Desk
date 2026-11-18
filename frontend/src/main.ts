import "./styles/app.css";
import { renderAuthView } from "./components/AuthView";
import { escapeHtml, extractErrorMessage } from "./components/dom";
import { openGroupModal } from "./components/GroupModal";
import { openHostModal } from "./components/HostModal";
import { renderInstanceSidebar } from "./components/InstanceSidebar";
import { openKeyVaultModal } from "./components/KeyVaultModal";
import { TerminalTabsManager } from "./components/TerminalTabs";
import { openTofuModal } from "./components/TofuModal";
import { openTunnelModal } from "./components/TunnelModal";
import { renderTunnelSidebarBody } from "./components/TunnelSidebar";
import { XftpDualPane } from "./components/XftpDualPane";
import { getLocale, setLocale, t, type Locale } from "./i18n";
import {
  desktopApi,
  type HostGroup,
  type HostInstance,
  type SSHKeySummary,
  type SSHTunnel,
  type TunnelRuntimeStatus,
  type UserProfile,
} from "./services/desktopApi";

const rootEl = document.getElementById("app") as HTMLElement;

let activeUser: UserProfile | null = null;
let sidebarTab: "hosts" | "tunnels" = "hosts";
let hosts: HostInstance[] = [];
let groups: HostGroup[] = [];
let keys: SSHKeySummary[] = [];
let tunnels: SSHTunnel[] = [];
const tunnelStatuses = new Map<string, TunnelRuntimeStatus>();
let searchFilter = "";
let envFilter = "ALL";
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
  collapsedGroups.clear();

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
    const [loadedHosts, loadedGroups, loadedKeys, loadedTunnels, loadedStatuses] =
      await Promise.all([
        desktopApi.hosts.list(),
        desktopApi.groups.list(),
        desktopApi.keys.list(),
        desktopApi.tunnels.list(),
        desktopApi.tunnels.listStatuses(),
      ]);
    hosts = loadedHosts ?? [];
    groups = loadedGroups ?? [];
    keys = loadedKeys ?? [];
    tunnels = loadedTunnels ?? [];
    tunnelStatuses.clear();
    for (const st of loadedStatuses ?? []) {
      tunnelStatuses.set(st.tunnelId, st);
    }
  } catch (err) {
    hosts = hosts ?? [];
    groups = groups ?? [];
    keys = keys ?? [];
    tunnels = tunnels ?? [];
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

  sidebarEl.innerHTML = `
    <div class="sidebar-tabs">
      <button id="stab-hosts" class="sidebar-tab-btn ${sidebarTab === "hosts" ? "active" : ""}">
        ${escapeHtml(copy.sidebar.tabInstances)} (${hosts.length})
      </button>
      <button id="stab-tunnels" class="sidebar-tab-btn ${sidebarTab === "tunnels" ? "active" : ""}">
        ${escapeHtml(copy.sidebar.tabTunnels)} (${activeCount > 0 ? `${activeCount}/` : ""}${tunnels.length})
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

  const prevSearch = bodyEl.querySelector<HTMLInputElement>("#host-search");
  const hadFocus = document.activeElement === prevSearch;
  const cursorPos = prevSearch?.selectionStart ?? null;

  renderInstanceSidebar(
    bodyEl,
    { hosts, groups, keys, search: searchFilter, envFilter, collapsedGroups },
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

  xftpPane = new XftpDualPane(xftpEl);
  terminalMgr = new TerminalTabsManager(
    tabsBar,
    stageEl,
    (sessionId) => {
      if (xftpEl.style.display !== "none") {
        void xftpPane?.setSession(sessionId);
      }
    },
    (visible) => {
      xftpEl.style.display = visible ? "flex" : "none";
      if (visible) {
        void xftpPane?.setSession(terminalMgr?.getActiveSessionId() ?? "");
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
