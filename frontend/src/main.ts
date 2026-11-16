import "./styles/app.css";
import { renderAuthView } from "./components/AuthView";
import { escapeHtml, extractErrorMessage } from "./components/dom";
import { openHostModal } from "./components/HostModal";
import { renderInstanceSidebar } from "./components/InstanceSidebar";
import { openKeyVaultModal } from "./components/KeyVaultModal";
import { TerminalTabsManager } from "./components/TerminalTabs";
import { openTofuModal } from "./components/TofuModal";
import { XftpDualPane } from "./components/XftpDualPane";
import { getLocale, setLocale, t, type Locale } from "./i18n";
import {
  desktopApi,
  type HostInstance,
  type SSHKeySummary,
  type UserProfile,
} from "./services/desktopApi";

const rootEl = document.getElementById("app") as HTMLElement;

let activeUser: UserProfile | null = null;
let hosts: HostInstance[] = [];
let keys: SSHKeySummary[] = [];
let searchFilter = "";
let envFilter = "ALL";
let terminalMgr: TerminalTabsManager | null = null;
let xftpPane: XftpDualPane | null = null;

desktopApi.terminal.onTOFUPrompt((challenge) => {
  openTofuModal(challenge);
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
    const [loadedHosts, loadedKeys] = await Promise.all([
      desktopApi.hosts.list(),
      desktopApi.keys.list(),
    ]);
    hosts = loadedHosts ?? [];
    keys = loadedKeys ?? [];
  } catch (err) {
    hosts = hosts ?? [];
    keys = keys ?? [];
    window.alert(extractErrorMessage(err));
  }
  const sidebarEl = rootEl.querySelector<HTMLElement>("#sidebar");
  if (sidebarEl) {
    drawSidebar(sidebarEl);
  }
}

function drawSidebar(sidebarEl: HTMLElement): void {
  renderInstanceSidebar(
    sidebarEl,
    { hosts, keys, search: searchFilter, envFilter },
    {
      onSearchChange: (q) => {
        searchFilter = q;
        drawSidebar(sidebarEl);
      },
      onEnvFilterChange: (env) => {
        envFilter = env;
        drawSidebar(sidebarEl);
      },
      onNewHost: () => {
        openHostModal(null, keys, () => {
          void refreshInventory();
        });
      },
      onConnectHost: (host) => {
        void connectHost(host);
      },
      onEditHost: (host) => {
        openHostModal(host, keys, () => {
          void refreshInventory();
        });
      },
      onDeleteHost: async (host) => {
        if (!window.confirm(t().common.confirmDelete)) return;
        await desktopApi.hosts.delete(host.id);
        await refreshInventory();
      },
    }
  );
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
