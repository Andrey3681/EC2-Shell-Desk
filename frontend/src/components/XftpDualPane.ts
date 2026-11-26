import { t } from "../i18n";
import {
  desktopApi,
  type FileEntry,
  type TransferTask,
} from "../services/desktopApi";
import { escapeHtml, extractErrorMessage, formatBytes } from "./dom";

export type XftpDisplayMode = "docked" | "floating" | "maximized";

const MODE_STORAGE_KEY = "ec2-shell-desk:xftp-mode";
const HEIGHT_STORAGE_KEY = "ec2-shell-desk:xftp-height";

const CODE_EXTS = new Set([
  "go", "ts", "tsx", "js", "jsx", "mjs", "cjs", "py", "rs", "sh", "bash",
  "zsh", "ps1", "sql", "html", "css", "scss", "json", "yml", "yaml", "xml",
  "md", "toml", "java", "c", "cpp", "h", "cs", "php", "rb", "dockerfile",
]);

const ARCHIVE_EXTS = new Set([
  "zip", "tar", "gz", "tgz", "bz2", "xz", "7z", "rar", "zst",
]);

const CONFIG_EXTS = new Set([
  "env", "conf", "cfg", "ini", "log", "pem", "key", "crt", "pub", "service",
]);

function renderEntryIcon(name: string, isDir: boolean): string {
  if (isDir) {
    return `<svg class="xftp-icon xftp-icon-dir" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M10 4H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-8l-2-2z"/></svg>`;
  }
  const lower = name.toLowerCase();
  const dotIdx = lower.lastIndexOf(".");
  const ext = dotIdx >= 0 ? lower.slice(dotIdx + 1) : lower;

  if (ARCHIVE_EXTS.has(ext)) {
    return `<svg class="xftp-icon xftp-icon-archive" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1z"/><path d="M10 12h4"/></svg>`;
  }
  if (CODE_EXTS.has(ext)) {
    return `<svg class="xftp-icon xftp-icon-code" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>`;
  }
  if (CONFIG_EXTS.has(ext) || lower.startsWith(".env")) {
    return `<svg class="xftp-icon xftp-icon-config" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h.01a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h.01a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v.01a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`;
  }
  return `<svg class="xftp-icon xftp-icon-file" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>`;
}

interface BreadcrumbSegment {
  label: string;
  targetPath: string;
}

function buildLocalBreadcrumbs(localPath: string): BreadcrumbSegment[] {
  const normalized = localPath.replace(/\//g, "\\").trim();
  if (!normalized) return [];
  const parts = normalized.split("\\").filter((p) => p.length > 0);
  const crumbs: BreadcrumbSegment[] = [];
  let acc = "";
  parts.forEach((part, idx) => {
    if (idx === 0 && part.endsWith(":")) {
      acc = `${part}\\`;
      crumbs.push({ label: acc, targetPath: acc });
    } else {
      acc = acc.endsWith("\\") ? `${acc}${part}` : `${acc}\\${part}`;
      crumbs.push({ label: part, targetPath: acc });
    }
  });
  return crumbs;
}

function buildRemoteBreadcrumbs(remotePath: string): BreadcrumbSegment[] {
  const normalized = remotePath.trim() || "/";
  const crumbs: BreadcrumbSegment[] = [{ label: "/", targetPath: "/" }];
  const parts = normalized.split("/").filter((p) => p.length > 0);
  let acc = "";
  for (const part of parts) {
    acc = `${acc}/${part}`;
    crumbs.push({ label: part, targetPath: acc });
  }
  return crumbs;
}

export class XftpDualPane {
  private container: HTMLElement;
  private onClose?: () => void;

  private sessionId = "";
  private contextLabel = "";

  private localRoots: string[] = [];
  private localPath = "";
  private remotePath = "";
  private localEntries: FileEntry[] = [];
  private remoteEntries: FileEntry[] = [];

  private selectedLocal = new Set<string>();
  private selectedRemote = new Set<string>();
  private localFilter = "";
  private remoteFilter = "";
  private editingLocalPath = false;
  private editingRemotePath = false;

  private displayMode: XftpDisplayMode = "docked";
  private dockedHeight = 360;
  private queueCollapsed = false;

  private transfers = new Map<string, TransferTask>();
  private errorMsg = "";
  private unsubProgress: (() => void) | null = null;

  constructor(container: HTMLElement, onClose?: () => void) {
    this.container = container;
    this.onClose = onClose;

    const savedMode = window.localStorage.getItem(MODE_STORAGE_KEY);
    if (savedMode === "docked" || savedMode === "floating" || savedMode === "maximized") {
      this.displayMode = savedMode;
    }
    const savedHeight = Number(window.localStorage.getItem(HEIGHT_STORAGE_KEY));
    if (Number.isFinite(savedHeight) && savedHeight >= 220 && savedHeight <= 900) {
      this.dockedHeight = savedHeight;
    }

    this.applyDisplayModeClasses();

    this.unsubProgress = desktopApi.sftp.onProgress((task) => {
      this.transfers.set(task.id, task);
      this.pruneFinishedTransfers(50);
      this.renderQueue();
      if (task.state === "completed") {
        void this.refreshBoth();
      }
    });
  }

  private pruneFinishedTransfers(maxFinished: number): void {
    const finishedIds: string[] = [];
    for (const [id, item] of this.transfers) {
      if (item.state !== "running" && item.state !== "pending") {
        finishedIds.push(id);
      }
    }
    const excess = finishedIds.length - maxFinished;
    for (let i = 0; i < excess; i++) {
      this.transfers.delete(finishedIds[i]);
    }
  }

  public async setSession(sessionId: string, contextLabel = ""): Promise<void> {
    const sessionChanged = this.sessionId !== sessionId;
    this.sessionId = sessionId;
    this.contextLabel = contextLabel;
    this.errorMsg = "";

    if (!sessionId) {
      this.remoteEntries = [];
      this.selectedRemote.clear();
      this.render();
      return;
    }

    try {
      if (!this.localPath) {
        const [roots, home] = await Promise.all([
          desktopApi.sftp.listLocalRoots(),
          desktopApi.sftp.localHomeDir(),
        ]);
        this.localRoots = roots;
        this.localPath = home || roots[0] || "C:\\";
      }
      if (sessionChanged || !this.remotePath) {
        this.remotePath = await desktopApi.sftp.remoteWorkingDir(sessionId);
        this.selectedRemote.clear();
        this.remoteFilter = "";
      }
      await this.refreshBoth();
    } catch (err) {
      this.errorMsg = extractErrorMessage(err);
      this.render();
    }
  }

  public setDisplayMode(mode: XftpDisplayMode): void {
    this.displayMode = mode;
    window.localStorage.setItem(MODE_STORAGE_KEY, mode);
    this.applyDisplayModeClasses();
    this.render();
  }

  private applyDisplayModeClasses(): void {
    this.container.classList.remove("xftp-mode-maximized", "xftp-mode-floating");
    if (this.displayMode === "maximized") {
      this.container.classList.add("xftp-mode-maximized");
      this.container.style.height = "";
      this.container.style.left = "";
      this.container.style.top = "";
    } else if (this.displayMode === "floating") {
      this.container.classList.add("xftp-mode-floating");
      this.container.style.height = "";
    } else {
      this.container.style.left = "";
      this.container.style.top = "";
      this.container.style.width = "";
      this.container.style.height = `${this.dockedHeight}px`;
    }
  }

  public dispose(): void {
    this.unsubProgress?.();
    this.unsubProgress = null;
  }

  public async refreshBoth(): Promise<void> {
    await Promise.all([this.loadLocal(), this.loadRemote()]);
    this.render();
  }

  private async loadLocal(): Promise<void> {
    if (!this.localPath) return;
    try {
      this.localEntries = await desktopApi.sftp.listLocalDir(this.localPath);
      const validPaths = new Set(this.localEntries.map((e) => e.path));
      for (const sel of Array.from(this.selectedLocal)) {
        if (!validPaths.has(sel)) this.selectedLocal.delete(sel);
      }
    } catch (err) {
      this.errorMsg = extractErrorMessage(err);
    }
  }

  private async loadRemote(): Promise<void> {
    if (!this.sessionId || !this.remotePath) return;
    try {
      this.remoteEntries = await desktopApi.sftp.listRemoteDir(
        this.sessionId,
        this.remotePath
      );
      const validPaths = new Set(this.remoteEntries.map((e) => e.path));
      for (const sel of Array.from(this.selectedRemote)) {
        if (!validPaths.has(sel)) this.selectedRemote.delete(sel);
      }
    } catch (err) {
      this.errorMsg = extractErrorMessage(err);
    }
  }

  private filteredLocalEntries(): FileEntry[] {
    const q = this.localFilter.trim().toLowerCase();
    if (!q) return this.localEntries;
    return this.localEntries.filter((e) => e.name.toLowerCase().includes(q));
  }

  private filteredRemoteEntries(): FileEntry[] {
    const q = this.remoteFilter.trim().toLowerCase();
    if (!q) return this.remoteEntries;
    return this.remoteEntries.filter((e) => e.name.toLowerCase().includes(q));
  }

  public render(): void {
    const copy = t();
    this.applyDisplayModeClasses();

    const activeEl = document.activeElement as HTMLInputElement | null;
    const focusId = activeEl?.id ?? "";
    const selStart = activeEl?.selectionStart ?? null;
    const selEnd = activeEl?.selectionEnd ?? null;

    const topBarHtml = `
      <div id="xftp-splitter" class="xftp-splitter" title="Resize"></div>
      <div id="xftp-topbar" class="xftp-topbar">
        <div class="xftp-topbar-left">
          <span class="xftp-topbar-title">${escapeHtml(copy.sftp.title)}</span>
          ${
            this.sessionId
              ? `<span class="xftp-context-pill" title="${escapeHtml(this.contextLabel || this.sessionId)}">${escapeHtml(this.contextLabel || this.sessionId)}</span>`
              : ""
          }
        </div>
        <div class="xftp-mode-group">
          <button type="button" class="xftp-mode-btn ${this.displayMode === "docked" ? "active" : ""}" data-xftp-mode="docked">${escapeHtml(copy.sftp.modeDocked)}</button>
          <button type="button" class="xftp-mode-btn ${this.displayMode === "floating" ? "active" : ""}" data-xftp-mode="floating">${escapeHtml(copy.sftp.modeFloating)}</button>
          <button type="button" class="xftp-mode-btn ${this.displayMode === "maximized" ? "active" : ""}" data-xftp-mode="maximized">${escapeHtml(copy.sftp.modeMaximized)}</button>
          ${this.onClose ? `<button type="button" id="xftp-close-btn" class="xftp-mode-btn" title="${escapeHtml(copy.common.close)}">&times;</button>` : ""}
        </div>
      </div>
    `;

    if (!this.sessionId) {
      this.container.innerHTML = `
        ${topBarHtml}
        <div style="padding:24px;color:var(--text-secondary);text-align:center;flex:1;display:flex;align-items:center;justify-content:center;">
          ${escapeHtml(copy.sftp.openSessionFirst)}
        </div>
      `;
      this.bindChromeEvents();
      return;
    }

    const visibleLocal = this.filteredLocalEntries();
    const visibleRemote = this.filteredRemoteEntries();
    const localCrumbs = buildLocalBreadcrumbs(this.localPath);
    const remoteCrumbs = buildRemoteBreadcrumbs(this.remotePath);
    const allLocalChecked =
      visibleLocal.length > 0 && visibleLocal.every((e) => this.selectedLocal.has(e.path));
    const allRemoteChecked =
      visibleRemote.length > 0 && visibleRemote.every((e) => this.selectedRemote.has(e.path));

    const remoteFilesSelected = Array.from(this.selectedRemote).filter((p) => {
      const entry = this.remoteEntries.find((e) => e.path === p);
      return entry && !entry.isDir;
    });

    this.container.innerHTML = `
      ${topBarHtml}
      ${this.errorMsg ? `<div class="error-banner" style="margin:6px 8px 0;">${escapeHtml(this.errorMsg)}</div>` : ""}
      <div class="xftp-panes">
        <!-- LOCAL PANE -->
        <div id="xftp-local-pane" class="xftp-pane" data-drop-hint="${escapeHtml(copy.sftp.dropToDownload)}">
          <div class="xftp-pane-header">
            <span class="xftp-pane-title">${escapeHtml(copy.sftp.localTitle)}</span>
            <input id="local-filter-input" class="xftp-filter-input" placeholder="${escapeHtml(copy.sftp.filterPlaceholder)}" value="${escapeHtml(this.localFilter)}" />
          </div>
          <div class="xftp-nav-row">
            <select id="local-root-sel" class="select mono" style="width:66px;padding:2px 5px;font-size:11px;">
              ${this.localRoots.map((r) => `<option value="${escapeHtml(r)}" ${this.localPath.toUpperCase().startsWith(r.toUpperCase()) ? "selected" : ""}>${escapeHtml(r)}</option>`).join("")}
            </select>
            <button type="button" id="local-up" class="btn btn-sm" title="${escapeHtml(copy.sftp.upDir)}">&uarr;</button>
            ${
              this.editingLocalPath
                ? `<input id="local-path-input" class="input mono" style="padding:2px 8px;flex:1;font-size:11px;" value="${escapeHtml(this.localPath)}" />`
                : `<div class="xftp-breadcrumbs" id="local-breadcrumbs">
                    ${localCrumbs
                      .map(
                        (c, i) =>
                          `${i > 0 ? `<span class="xftp-crumb-sep">&rsaquo;</span>` : ""}<button type="button" class="xftp-crumb ${i === localCrumbs.length - 1 ? "current" : ""}" data-local-crumb="${escapeHtml(c.targetPath)}">${escapeHtml(c.label)}</button>`
                      )
                      .join("")}
                  </div>`
            }
            <button type="button" id="local-edit-path" class="btn btn-sm" title="${escapeHtml(copy.sftp.editPath)}">&#9998;</button>
          </div>
          <div class="xftp-actionbar">
            <button type="button" id="local-upload-folder" class="btn btn-primary btn-sm">${escapeHtml(copy.sftp.uploadFolder)}</button>
            <button type="button" id="local-upload-pick" class="btn btn-sm">${escapeHtml(copy.sftp.uploadFiles)}</button>
            <button type="button" id="local-upload-sel" class="btn btn-success btn-sm" ${this.selectedLocal.size === 0 ? "disabled" : ""}>${escapeHtml(copy.sftp.uploadSelected)} &rarr;</button>
            <button type="button" id="local-mkdir" class="btn btn-sm">${escapeHtml(copy.sftp.newFolder)}</button>
            ${this.selectedLocal.size > 0 ? `<span class="xftp-sel-badge">${this.selectedLocal.size} ${escapeHtml(copy.sftp.selectedCount)}</span>` : ""}
          </div>
          <div class="xftp-table-wrap">
            ${
              visibleLocal.length === 0
                ? `<div class="xftp-empty">${escapeHtml(copy.sftp.emptyDir)}</div>`
                : `<table class="xftp-table">
                    <thead>
                      <tr>
                        <th class="xftp-check-col"><input type="checkbox" id="local-check-all" ${allLocalChecked ? "checked" : ""} /></th>
                        <th>${escapeHtml(copy.sftp.colName)}</th>
                        <th class="xftp-size-col">${escapeHtml(copy.sftp.colSize)}</th>
                      </tr>
                    </thead>
                    <tbody>
                      ${visibleLocal
                        .map((e) => {
                          const selected = this.selectedLocal.has(e.path);
                          return `
                            <tr class="${selected ? "xftp-row-selected" : ""}" draggable="true" data-local-entry="${escapeHtml(e.path)}" data-is-dir="${e.isDir}">
                              <td class="xftp-check-col"><input type="checkbox" data-local-check="${escapeHtml(e.path)}" ${selected ? "checked" : ""} /></td>
                              <td>
                                <div class="xftp-name-cell">
                                  ${renderEntryIcon(e.name, e.isDir)}
                                  <span class="xftp-name-text mono" title="${escapeHtml(e.name)}">${escapeHtml(e.name)}</span>
                                </div>
                              </td>
                              <td class="xftp-size-col mono">${e.isDir ? "--" : escapeHtml(formatBytes(e.size))}</td>
                            </tr>
                          `;
                        })
                        .join("")}
                    </tbody>
                  </table>`
            }
          </div>
        </div>

        <!-- REMOTE PANE -->
        <div id="xftp-remote-pane" class="xftp-pane" data-drop-hint="${escapeHtml(copy.sftp.dropToUpload)}">
          <div class="xftp-pane-header">
            <span class="xftp-pane-title">${escapeHtml(copy.sftp.remoteTitle)}</span>
            <input id="remote-filter-input" class="xftp-filter-input" placeholder="${escapeHtml(copy.sftp.filterPlaceholder)}" value="${escapeHtml(this.remoteFilter)}" />
          </div>
          <div class="xftp-nav-row">
            <button type="button" id="remote-up" class="btn btn-sm" title="${escapeHtml(copy.sftp.upDir)}">&uarr;</button>
            ${
              this.editingRemotePath
                ? `<input id="remote-path-input" class="input mono" style="padding:2px 8px;flex:1;font-size:11px;" value="${escapeHtml(this.remotePath)}" />`
                : `<div class="xftp-breadcrumbs" id="remote-breadcrumbs">
                    ${remoteCrumbs
                      .map(
                        (c, i) =>
                          `${i > 0 ? `<span class="xftp-crumb-sep">&rsaquo;</span>` : ""}<button type="button" class="xftp-crumb ${i === remoteCrumbs.length - 1 ? "current" : ""}" data-remote-crumb="${escapeHtml(c.targetPath)}">${escapeHtml(c.label)}</button>`
                      )
                      .join("")}
                  </div>`
            }
            <button type="button" id="remote-edit-path" class="btn btn-sm" title="${escapeHtml(copy.sftp.editPath)}">&#9998;</button>
          </div>
          <div class="xftp-actionbar">
            <button type="button" id="remote-download-sel" class="btn btn-success btn-sm" ${remoteFilesSelected.length === 0 ? "disabled" : ""}>&larr; ${escapeHtml(copy.sftp.download)}</button>
            <button type="button" id="remote-mkdir" class="btn btn-sm">${escapeHtml(copy.sftp.newFolder)}</button>
            <button type="button" id="remote-rename-sel" class="btn btn-sm" ${this.selectedRemote.size !== 1 ? "disabled" : ""}>${escapeHtml(copy.sftp.rename)}</button>
            <button type="button" id="remote-delete-sel" class="btn btn-danger btn-sm" ${this.selectedRemote.size === 0 ? "disabled" : ""}>${escapeHtml(copy.sftp.delete)}</button>
            <button type="button" id="remote-refresh" class="btn btn-sm" title="${escapeHtml(copy.common.refresh)}">&#8635;</button>
            ${this.selectedRemote.size > 0 ? `<span class="xftp-sel-badge">${this.selectedRemote.size} ${escapeHtml(copy.sftp.selectedCount)}</span>` : ""}
          </div>
          <div class="xftp-table-wrap">
            ${
              visibleRemote.length === 0
                ? `<div class="xftp-empty">${escapeHtml(copy.sftp.emptyDir)}</div>`
                : `<table class="xftp-table">
                    <thead>
                      <tr>
                        <th class="xftp-check-col"><input type="checkbox" id="remote-check-all" ${allRemoteChecked ? "checked" : ""} /></th>
                        <th>${escapeHtml(copy.sftp.colName)}</th>
                        <th class="xftp-perms-col">${escapeHtml(copy.sftp.colPerms)}</th>
                        <th class="xftp-size-col">${escapeHtml(copy.sftp.colSize)}</th>
                      </tr>
                    </thead>
                    <tbody>
                      ${visibleRemote
                        .map((e) => {
                          const selected = this.selectedRemote.has(e.path);
                          return `
                            <tr class="${selected ? "xftp-row-selected" : ""}" draggable="true" data-remote-entry="${escapeHtml(e.path)}" data-is-dir="${e.isDir}">
                              <td class="xftp-check-col"><input type="checkbox" data-remote-check="${escapeHtml(e.path)}" ${selected ? "checked" : ""} /></td>
                              <td>
                                <div class="xftp-name-cell">
                                  ${renderEntryIcon(e.name, e.isDir)}
                                  <span class="xftp-name-text mono" title="${escapeHtml(e.name)}">${escapeHtml(e.name)}</span>
                                </div>
                              </td>
                              <td class="xftp-perms-col mono">${escapeHtml(e.permission || "")}</td>
                              <td class="xftp-size-col mono">${e.isDir ? "--" : escapeHtml(formatBytes(e.size))}</td>
                            </tr>
                          `;
                        })
                        .join("")}
                    </tbody>
                  </table>`
            }
          </div>
        </div>
      </div>

      <div class="xftp-queue-wrap">
        <div id="xftp-queue-toggle" class="xftp-queue-header">
          <span id="xftp-queue-summary">${escapeHtml(copy.sftp.queueTitle)}</span>
          <span>${this.queueCollapsed ? "&#9650;" : "&#9660;"}</span>
        </div>
        <div id="xftp-queue" class="xftp-queue ${this.queueCollapsed ? "collapsed" : ""}"></div>
      </div>
    `;

    this.bindChromeEvents();
    this.bindPaneEvents();
    this.renderQueue();

    if (focusId) {
      const nextFocus = this.container.querySelector<HTMLInputElement>(`#${focusId}`);
      if (nextFocus) {
        nextFocus.focus();
        if (selStart !== null && selEnd !== null) {
          nextFocus.setSelectionRange(selStart, selEnd);
        }
      }
    }
  }

  private renderQueue(): void {
    const queueEl = this.container.querySelector<HTMLElement>("#xftp-queue");
    const summaryEl = this.container.querySelector<HTMLElement>("#xftp-queue-summary");
    if (!queueEl) return;
    const copy = t();
    const allList = Array.from(this.transfers.values());
    const activeCount = allList.filter((item) => item.state === "running").length;

    if (summaryEl) {
      summaryEl.textContent =
        activeCount > 0
          ? `${copy.sftp.queueTitle} (${activeCount})`
          : copy.sftp.queueTitle;
    }

    const list = allList.slice(-8).reverse();
    if (list.length === 0) {
      queueEl.innerHTML = `<div style="color:var(--text-secondary);font-size:11px;">${escapeHtml(copy.sftp.noTransfers)}</div>`;
      return;
    }

    queueEl.innerHTML = list
      .map((item) => {
        const pct =
          item.totalBytes > 0
            ? Math.min(100, Math.round((item.bytesTransferred / item.totalBytes) * 100))
            : item.state === "completed"
              ? 100
              : 0;
        const name = item.direction === "upload" ? item.localPath : item.remotePath;
        const badgeCls =
          item.state === "completed"
            ? "badge-dev"
            : item.state === "failed" || item.state === "cancelled"
              ? "badge-prod"
              : "badge-staging";
        return `
          <div class="queue-row">
            <span class="badge ${badgeCls}">${escapeHtml(item.direction.toUpperCase())} ${escapeHtml(item.state)}</span>
            <span class="mono" style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;" title="${escapeHtml(name)}">${escapeHtml(name)}</span>
            <div class="progress-bar"><div class="progress-fill" style="width:${pct}%"></div></div>
            <span class="mono" style="width:36px;text-align:right;">${pct}%</span>
            ${item.state === "running" ? `<button type="button" class="btn btn-danger btn-sm" data-cancel-tx="${escapeHtml(item.id)}">${escapeHtml(copy.common.cancel)}</button>` : ""}
          </div>
        `;
      })
      .join("");

    queueEl.querySelectorAll<HTMLButtonElement>("[data-cancel-tx]").forEach((btn) => {
      btn.addEventListener("click", () => {
        void desktopApi.sftp.cancelTransfer(btn.dataset.cancelTx ?? "");
      });
    });
  }

  private bindChromeEvents(): void {
    this.container.querySelectorAll<HTMLButtonElement>("[data-xftp-mode]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const mode = btn.dataset.xftpMode as XftpDisplayMode;
        if (mode) this.setDisplayMode(mode);
      });
    });

    this.container.querySelector("#xftp-close-btn")?.addEventListener("click", () => {
      this.onClose?.();
    });

    // Vertical resize splitter in docked mode
    const splitter = this.container.querySelector<HTMLElement>("#xftp-splitter");
    splitter?.addEventListener("mousedown", (startEv) => {
      if (this.displayMode !== "docked") return;
      startEv.preventDefault();
      splitter.classList.add("active");
      const startY = startEv.clientY;
      const startHeight = this.container.getBoundingClientRect().height;

      const onMove = (moveEv: MouseEvent) => {
        const delta = startY - moveEv.clientY;
        const maxH = Math.round(window.innerHeight * 0.82);
        const nextH = Math.max(220, Math.min(maxH, Math.round(startHeight + delta)));
        this.dockedHeight = nextH;
        this.container.style.height = `${nextH}px`;
      };

      const onUp = () => {
        splitter.classList.remove("active");
        window.localStorage.setItem(HEIGHT_STORAGE_KEY, String(this.dockedHeight));
        window.removeEventListener("mousemove", onMove);
        window.removeEventListener("mouseup", onUp);
      };

      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onUp);
    });

    // Drag to reposition floating panel
    const topbar = this.container.querySelector<HTMLElement>("#xftp-topbar");
    topbar?.addEventListener("mousedown", (startEv) => {
      if (this.displayMode !== "floating") return;
      if ((startEv.target as HTMLElement).closest("button")) return;
      startEv.preventDefault();
      const rect = this.container.getBoundingClientRect();
      const offsetX = startEv.clientX - rect.left;
      const offsetY = startEv.clientY - rect.top;

      const onMove = (moveEv: MouseEvent) => {
        const maxLeft = Math.max(0, window.innerWidth - 160);
        const maxTop = Math.max(0, window.innerHeight - 80);
        const left = Math.max(0, Math.min(maxLeft, moveEv.clientX - offsetX));
        const top = Math.max(0, Math.min(maxTop, moveEv.clientY - offsetY));
        this.container.style.left = `${left}px`;
        this.container.style.top = `${top}px`;
      };

      const onUp = () => {
        window.removeEventListener("mousemove", onMove);
        window.removeEventListener("mouseup", onUp);
      };

      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onUp);
    });
  }

  private bindPaneEvents(): void {
    const copy = t();

    // Queue collapse toggle
    this.container.querySelector("#xftp-queue-toggle")?.addEventListener("click", () => {
      this.queueCollapsed = !this.queueCollapsed;
      this.container.querySelector("#xftp-queue")?.classList.toggle("collapsed", this.queueCollapsed);
      this.render();
    });

    // Filters
    this.container.querySelector<HTMLInputElement>("#local-filter-input")?.addEventListener("input", (e) => {
      this.localFilter = (e.target as HTMLInputElement).value;
      this.render();
    });
    this.container.querySelector<HTMLInputElement>("#remote-filter-input")?.addEventListener("input", (e) => {
      this.remoteFilter = (e.target as HTMLInputElement).value;
      this.render();
    });

    // Local navigation
    this.container.querySelector<HTMLSelectElement>("#local-root-sel")?.addEventListener("change", (e) => {
      this.localPath = (e.target as HTMLSelectElement).value;
      this.selectedLocal.clear();
      this.localFilter = "";
      void this.refreshBoth();
    });

    this.container.querySelector("#local-up")?.addEventListener("click", () => {
      const parent = this.localPath.replace(/[\\/][^\\/]+[\\/]?$/, "");
      if (parent) {
        this.localPath = parent.endsWith(":") ? `${parent}\\` : parent;
        this.selectedLocal.clear();
        void this.refreshBoth();
      }
    });

    this.container.querySelector("#local-edit-path")?.addEventListener("click", () => {
      this.editingLocalPath = !this.editingLocalPath;
      this.render();
      if (this.editingLocalPath) {
        const inp = this.container.querySelector<HTMLInputElement>("#local-path-input");
        inp?.focus();
        inp?.select();
      }
    });

    this.container.querySelector<HTMLInputElement>("#local-path-input")?.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        this.localPath = (e.target as HTMLInputElement).value.trim();
        this.editingLocalPath = false;
        this.selectedLocal.clear();
        void this.refreshBoth();
      } else if (e.key === "Escape") {
        this.editingLocalPath = false;
        this.render();
      }
    });

    this.container.querySelectorAll<HTMLButtonElement>("[data-local-crumb]").forEach((btn) => {
      btn.addEventListener("click", () => {
        this.localPath = btn.dataset.localCrumb ?? this.localPath;
        this.selectedLocal.clear();
        void this.refreshBoth();
      });
    });

    // Remote navigation
    this.container.querySelector("#remote-up")?.addEventListener("click", () => {
      if (this.remotePath === "/") return;
      const parent = this.remotePath.replace(/\/[^/]+\/?$/, "") || "/";
      this.remotePath = parent;
      this.selectedRemote.clear();
      void this.refreshBoth();
    });

    this.container.querySelector("#remote-edit-path")?.addEventListener("click", () => {
      this.editingRemotePath = !this.editingRemotePath;
      this.render();
      if (this.editingRemotePath) {
        const inp = this.container.querySelector<HTMLInputElement>("#remote-path-input");
        inp?.focus();
        inp?.select();
      }
    });

    this.container.querySelector<HTMLInputElement>("#remote-path-input")?.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        this.remotePath = (e.target as HTMLInputElement).value.trim() || "/";
        this.editingRemotePath = false;
        this.selectedRemote.clear();
        void this.refreshBoth();
      } else if (e.key === "Escape") {
        this.editingRemotePath = false;
        this.render();
      }
    });

    this.container.querySelectorAll<HTMLButtonElement>("[data-remote-crumb]").forEach((btn) => {
      btn.addEventListener("click", () => {
        this.remotePath = btn.dataset.remoteCrumb ?? "/";
        this.selectedRemote.clear();
        void this.refreshBoth();
      });
    });

    // Local actions
    this.container.querySelector("#local-mkdir")?.addEventListener("click", async () => {
      const name = window.prompt(copy.sftp.folderNamePrompt);
      if (!name?.trim()) return;
      const sep = this.localPath.endsWith("\\") ? "" : "\\";
      try {
        await desktopApi.sftp.makeLocalDir(`${this.localPath}${sep}${name.trim()}`);
        await this.refreshBoth();
      } catch (err) {
        this.errorMsg = extractErrorMessage(err);
        this.render();
      }
    });

    this.container.querySelector("#local-upload-pick")?.addEventListener("click", async () => {
      try {
        const files = await desktopApi.sftp.pickLocalFilesToUpload();
        for (const f of files) {
          await desktopApi.sftp.startUpload(this.sessionId, f, this.remotePath);
        }
      } catch (err) {
        this.errorMsg = extractErrorMessage(err);
        this.render();
      }
    });

    this.container.querySelector("#local-upload-folder")?.addEventListener("click", async () => {
      try {
        const dirPath = await desktopApi.sftp.pickLocalFolderToUpload();
        if (dirPath) {
          await desktopApi.sftp.startUploadDirectory(this.sessionId, dirPath, this.remotePath);
        }
      } catch (err) {
        this.errorMsg = extractErrorMessage(err);
        this.render();
      }
    });

    this.container.querySelector("#local-upload-sel")?.addEventListener("click", async () => {
      await this.uploadLocalPaths(Array.from(this.selectedLocal));
    });

    // Remote actions
    this.container.querySelector("#remote-refresh")?.addEventListener("click", () => {
      void this.refreshBoth();
    });

    this.container.querySelector("#remote-mkdir")?.addEventListener("click", async () => {
      const name = window.prompt(copy.sftp.folderNamePrompt);
      if (!name?.trim()) return;
      const sep = this.remotePath.endsWith("/") ? "" : "/";
      try {
        await desktopApi.sftp.makeRemoteDir(this.sessionId, `${this.remotePath}${sep}${name.trim()}`);
        await this.refreshBoth();
      } catch (err) {
        this.errorMsg = extractErrorMessage(err);
        this.render();
      }
    });

    this.container.querySelector("#remote-download-sel")?.addEventListener("click", async () => {
      await this.downloadRemotePaths(Array.from(this.selectedRemote));
    });

    this.container.querySelector("#remote-rename-sel")?.addEventListener("click", async () => {
      if (this.selectedRemote.size !== 1) return;
      const oldPath = Array.from(this.selectedRemote)[0];
      const base = oldPath.split("/").pop() ?? "";
      const next = window.prompt(copy.sftp.renamePrompt, base);
      if (!next?.trim() || next.trim() === base) return;
      const dir = oldPath.replace(/\/[^/]+$/, "") || "/";
      const newPath = dir === "/" ? `/${next.trim()}` : `${dir}/${next.trim()}`;
      try {
        await desktopApi.sftp.renameRemote(this.sessionId, oldPath, newPath);
        this.selectedRemote.clear();
        await this.refreshBoth();
      } catch (err) {
        this.errorMsg = extractErrorMessage(err);
        this.render();
      }
    });

    this.container.querySelector("#remote-delete-sel")?.addEventListener("click", async () => {
      if (this.selectedRemote.size === 0) return;
      if (!window.confirm(copy.sftp.confirmDelete)) return;
      try {
        for (const target of Array.from(this.selectedRemote)) {
          const entry = this.remoteEntries.find((e) => e.path === target);
          await desktopApi.sftp.deleteRemote(this.sessionId, target, Boolean(entry?.isDir));
        }
        this.selectedRemote.clear();
        await this.refreshBoth();
      } catch (err) {
        this.errorMsg = extractErrorMessage(err);
        this.render();
      }
    });

    // Check-all boxes
    this.container.querySelector<HTMLInputElement>("#local-check-all")?.addEventListener("change", (e) => {
      const checked = (e.target as HTMLInputElement).checked;
      for (const entry of this.filteredLocalEntries()) {
        if (checked) this.selectedLocal.add(entry.path);
        else this.selectedLocal.delete(entry.path);
      }
      this.render();
    });

    this.container.querySelector<HTMLInputElement>("#remote-check-all")?.addEventListener("change", (e) => {
      const checked = (e.target as HTMLInputElement).checked;
      for (const entry of this.filteredRemoteEntries()) {
        if (checked) this.selectedRemote.add(entry.path);
        else this.selectedRemote.delete(entry.path);
      }
      this.render();
    });

    // Local rows: click to select, dblclick to open/upload, dragstart
    this.container.querySelectorAll<HTMLTableRowElement>("tr[data-local-entry]").forEach((row) => {
      const path = row.dataset.localEntry ?? "";
      const isDir = row.dataset.isDir === "true";

      row.addEventListener("click", (ev) => {
        const target = ev.target as HTMLElement;
        if (target.tagName === "INPUT") {
          if ((target as HTMLInputElement).checked) this.selectedLocal.add(path);
          else this.selectedLocal.delete(path);
          this.render();
          return;
        }
        if (ev.ctrlKey || ev.metaKey) {
          if (this.selectedLocal.has(path)) this.selectedLocal.delete(path);
          else this.selectedLocal.add(path);
        } else {
          this.selectedLocal.clear();
          this.selectedLocal.add(path);
        }
        this.render();
      });

      row.addEventListener("dblclick", async () => {
        if (isDir) {
          this.localPath = path;
          this.selectedLocal.clear();
          this.localFilter = "";
          await this.refreshBoth();
        } else {
          await this.uploadLocalPaths([path]);
        }
      });

      row.addEventListener("dragstart", (ev) => {
        if (!this.selectedLocal.has(path)) {
          this.selectedLocal.clear();
          this.selectedLocal.add(path);
        }
        ev.dataTransfer?.setData("application/x-xftp-local", JSON.stringify(Array.from(this.selectedLocal)));
        if (ev.dataTransfer) ev.dataTransfer.effectAllowed = "copy";
      });
    });

    // Remote rows: click to select, dblclick to open/download, dragstart
    this.container.querySelectorAll<HTMLTableRowElement>("tr[data-remote-entry]").forEach((row) => {
      const path = row.dataset.remoteEntry ?? "";
      const isDir = row.dataset.isDir === "true";

      row.addEventListener("click", (ev) => {
        const target = ev.target as HTMLElement;
        if (target.tagName === "INPUT") {
          if ((target as HTMLInputElement).checked) this.selectedRemote.add(path);
          else this.selectedRemote.delete(path);
          this.render();
          return;
        }
        if (ev.ctrlKey || ev.metaKey) {
          if (this.selectedRemote.has(path)) this.selectedRemote.delete(path);
          else this.selectedRemote.add(path);
        } else {
          this.selectedRemote.clear();
          this.selectedRemote.add(path);
        }
        this.render();
      });

      row.addEventListener("dblclick", async () => {
        if (isDir) {
          this.remotePath = path;
          this.selectedRemote.clear();
          this.remoteFilter = "";
          await this.refreshBoth();
        } else {
          await this.downloadRemotePaths([path]);
        }
      });

      row.addEventListener("dragstart", (ev) => {
        if (!this.selectedRemote.has(path)) {
          this.selectedRemote.clear();
          this.selectedRemote.add(path);
        }
        ev.dataTransfer?.setData("application/x-xftp-remote", JSON.stringify(Array.from(this.selectedRemote)));
        if (ev.dataTransfer) ev.dataTransfer.effectAllowed = "copy";
      });
    });

    // Drag & Drop targets
    const remotePane = this.container.querySelector<HTMLElement>("#xftp-remote-pane");
    remotePane?.addEventListener("dragover", (ev) => {
      if (ev.dataTransfer?.types.includes("application/x-xftp-local")) {
        ev.preventDefault();
        remotePane.classList.add("xftp-drag-over");
      }
    });
    remotePane?.addEventListener("dragleave", () => {
      remotePane.classList.remove("xftp-drag-over");
    });
    remotePane?.addEventListener("drop", async (ev) => {
      ev.preventDefault();
      remotePane.classList.remove("xftp-drag-over");
      const raw = ev.dataTransfer?.getData("application/x-xftp-local");
      if (!raw) return;
      try {
        const paths = JSON.parse(raw) as string[];
        await this.uploadLocalPaths(paths);
      } catch {
        // Ignore invalid drag payload
      }
    });

    const localPane = this.container.querySelector<HTMLElement>("#xftp-local-pane");
    localPane?.addEventListener("dragover", (ev) => {
      if (ev.dataTransfer?.types.includes("application/x-xftp-remote")) {
        ev.preventDefault();
        localPane.classList.add("xftp-drag-over");
      }
    });
    localPane?.addEventListener("dragleave", () => {
      localPane.classList.remove("xftp-drag-over");
    });
    localPane?.addEventListener("drop", async (ev) => {
      ev.preventDefault();
      localPane.classList.remove("xftp-drag-over");
      const raw = ev.dataTransfer?.getData("application/x-xftp-remote");
      if (!raw) return;
      try {
        const paths = JSON.parse(raw) as string[];
        await this.downloadRemotePaths(paths);
      } catch {
        // Ignore invalid drag payload
      }
    });
  }

  private async uploadLocalPaths(paths: string[]): Promise<void> {
    if (!this.sessionId || paths.length === 0) return;
    try {
      for (const p of paths) {
        const entry = this.localEntries.find((e) => e.path === p);
        if (entry?.isDir) {
          await desktopApi.sftp.startUploadDirectory(this.sessionId, p, this.remotePath);
        } else {
          await desktopApi.sftp.startUpload(this.sessionId, p, this.remotePath);
        }
      }
    } catch (err) {
      this.errorMsg = extractErrorMessage(err);
      this.render();
    }
  }

  private async downloadRemotePaths(paths: string[]): Promise<void> {
    if (!this.sessionId || paths.length === 0) return;
    try {
      for (const p of paths) {
        const entry = this.remoteEntries.find((e) => e.path === p);
        if (entry?.isDir) continue;
        await desktopApi.sftp.startDownload(this.sessionId, p, this.localPath);
      }
    } catch (err) {
      this.errorMsg = extractErrorMessage(err);
      this.render();
    }
  }
}
