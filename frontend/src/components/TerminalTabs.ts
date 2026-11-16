import "@xterm/xterm/css/xterm.css";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import { t } from "../i18n";
import { desktopApi, type SessionInfo } from "../services/desktopApi";
import { escapeHtml } from "./dom";

interface TabRecord {
  info: SessionInfo;
  term: Terminal;
  fit: FitAddon;
  mountEl: HTMLDivElement;
  closed: boolean;
  unsubData: () => void;
  unsubClosed: () => void;
}

export class TerminalTabsManager {
  private tabsBar: HTMLElement;
  private stageEl: HTMLElement;
  private tabs = new Map<string, TabRecord>();
  private activeSessionId = "";
  private sftpVisible = false;
  private resizeObserver: ResizeObserver;
  private onActiveChanged: (sessionId: string) => void;
  private onToggleSftp: (visible: boolean) => void;

  constructor(
    tabsBar: HTMLElement,
    stageEl: HTMLElement,
    onActiveChanged: (sessionId: string) => void,
    onToggleSftp: (visible: boolean) => void
  ) {
    this.tabsBar = tabsBar;
    this.stageEl = stageEl;
    this.onActiveChanged = onActiveChanged;
    this.onToggleSftp = onToggleSftp;

    this.resizeObserver = new ResizeObserver(() => {
      this.fitActive();
    });
    this.resizeObserver.observe(this.stageEl);
    this.render();
  }

  public getActiveSessionId(): string {
    return this.activeSessionId;
  }

  public estimateDimensions(): { cols: number; rows: number } {
    const w = this.stageEl.clientWidth || 800;
    const h = this.stageEl.clientHeight || 420;
    return {
      cols: Math.max(40, Math.floor(w / 9)),
      rows: Math.max(12, Math.floor(h / 18)),
    };
  }

  public addSession(info: SessionInfo): void {
    const mountEl = document.createElement("div");
    mountEl.className = "terminal-instance";
    this.stageEl.appendChild(mountEl);

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: '"Cascadia Code", "Consolas", monospace',
      theme: {
        background: "#000000",
        foreground: "#e2e8f0",
        cursor: "#38bdf8",
      },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(mountEl);

    term.onData((data) => {
      void desktopApi.terminal.write(info.sessionId, data);
    });

    const unsubData = desktopApi.terminal.onData(info.sessionId, (bytes) => {
      term.write(bytes);
    });

    const unsubClosed = desktopApi.terminal.onClosed(info.sessionId, (ev) => {
      const rec = this.tabs.get(info.sessionId);
      if (rec) {
        rec.closed = true;
        term.writeln(`\r\n\x1b[31m[${t().terminal.disconnected}: ${ev.reason}]\x1b[0m`);
        this.render();
      }
    });

    this.tabs.set(info.sessionId, {
      info,
      term,
      fit,
      mountEl,
      closed: false,
      unsubData,
      unsubClosed,
    });

    this.activate(info.sessionId);
  }

  public activate(sessionId: string): void {
    this.activeSessionId = sessionId;
    for (const [id, rec] of this.tabs) {
      rec.mountEl.classList.toggle("active", id === sessionId);
    }
    this.render();
    this.fitActive();
    this.tabs.get(sessionId)?.term.focus();
    this.onActiveChanged(sessionId);
  }

  public async closeTab(sessionId: string): Promise<void> {
    const rec = this.tabs.get(sessionId);
    if (!rec) return;

    rec.unsubData();
    rec.unsubClosed();
    rec.term.dispose();
    rec.mountEl.remove();
    this.tabs.delete(sessionId);

    try {
      await desktopApi.terminal.closeSession(sessionId);
    } catch {
      // Session may already be closed remotely.
    }

    if (this.activeSessionId === sessionId) {
      const next = this.tabs.keys().next().value ?? "";
      this.activeSessionId = next;
      if (next) {
        this.activate(next);
      } else {
        this.render();
        this.onActiveChanged("");
      }
    } else {
      this.render();
    }
  }

  public disposeAll(): void {
    for (const rec of this.tabs.values()) {
      rec.unsubData();
      rec.unsubClosed();
      rec.term.dispose();
      rec.mountEl.remove();
    }
    this.tabs.clear();
    this.activeSessionId = "";
    this.render();
  }

  public fitActive(): void {
    const rec = this.tabs.get(this.activeSessionId);
    if (!rec) return;
    try {
      rec.fit.fit();
      if (!rec.closed && rec.term.cols > 0 && rec.term.rows > 0) {
        void desktopApi.terminal.resize(rec.info.sessionId, rec.term.cols, rec.term.rows);
      }
    } catch {
      // Stage hidden or zero-sized during layout transitions.
    }
  }

  public render(): void {
    const copy = t();
    const welcome = this.stageEl.querySelector<HTMLElement>(".welcome-pane");
    if (welcome) {
      welcome.style.display = this.tabs.size === 0 ? "flex" : "none";
    }

    const tabHtml = Array.from(this.tabs.values())
      .map(
        (rec) => `
      <div class="tab-item ${rec.info.sessionId === this.activeSessionId ? "active" : ""}" data-tab="${escapeHtml(rec.info.sessionId)}">
        <span class="status-dot ${rec.closed ? "closed" : ""}"></span>
        <span>${escapeHtml(rec.info.hostAlias)} (${escapeHtml(rec.info.sshUsername)})</span>
        <button class="btn btn-sm" style="padding:0 5px;border:none;background:transparent;" data-close-tab="${escapeHtml(rec.info.sessionId)}">&times;</button>
      </div>`
      )
      .join("");

    this.tabsBar.innerHTML = `
      <div class="tabs-list">${tabHtml}</div>
      <button id="btn-toggle-sftp" class="btn btn-sm ${this.sftpVisible ? "btn-primary" : ""}" ${this.activeSessionId ? "" : "disabled"}>
        ${escapeHtml(copy.terminal.toggleSftp)}
      </button>
    `;

    this.tabsBar.querySelectorAll<HTMLElement>("[data-tab]").forEach((el) => {
      el.addEventListener("click", () => {
        this.activate(el.dataset.tab ?? "");
      });
    });

    this.tabsBar.querySelectorAll<HTMLButtonElement>("[data-close-tab]").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        void this.closeTab(btn.dataset.closeTab ?? "");
      });
    });

    this.tabsBar.querySelector("#btn-toggle-sftp")?.addEventListener("click", () => {
      this.sftpVisible = !this.sftpVisible;
      this.render();
      this.onToggleSftp(this.sftpVisible);
      requestAnimationFrame(() => this.fitActive());
    });
  }
}
