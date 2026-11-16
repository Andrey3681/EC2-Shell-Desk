import { t } from "../i18n";
import {
  desktopApi,
  type FileEntry,
  type TransferTask,
} from "../services/desktopApi";
import { escapeHtml, extractErrorMessage, formatBytes } from "./dom";

export class XftpDualPane {
  private container: HTMLElement;
  private sessionId = "";
  private localRoots: string[] = [];
  private localPath = "";
  private remotePath = "";
  private localEntries: FileEntry[] = [];
  private remoteEntries: FileEntry[] = [];
  private transfers = new Map<string, TransferTask>();
  private errorMsg = "";
  private unsubProgress: (() => void) | null = null;

  constructor(container: HTMLElement) {
    this.container = container;
    this.unsubProgress = desktopApi.sftp.onProgress((task) => {
      this.transfers.set(task.id, task);
      this.renderQueue();
      if (task.state === "completed") {
        void this.refreshBoth();
      }
    });
  }

  public async setSession(sessionId: string): Promise<void> {
    this.sessionId = sessionId;
    this.errorMsg = "";
    if (!sessionId) {
      this.remoteEntries = [];
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
      this.remotePath = await desktopApi.sftp.remoteWorkingDir(sessionId);
      await this.refreshBoth();
    } catch (err) {
      this.errorMsg = extractErrorMessage(err);
      this.render();
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
    } catch (err) {
      this.errorMsg = extractErrorMessage(err);
    }
  }

  public render(): void {
    const copy = t();
    if (!this.sessionId) {
      this.container.innerHTML = `<div style="padding:18px;color:var(--text-secondary);text-align:center;">${escapeHtml(copy.sftp.openSessionFirst)}</div>`;
      return;
    }

    this.container.innerHTML = `
      ${this.errorMsg ? `<div class="error-banner" style="margin:6px 8px 0;">${escapeHtml(this.errorMsg)}</div>` : ""}
      <div class="xftp-panes">
        <div class="xftp-pane">
          <div class="xftp-toolbar">
            <strong style="font-size:12px;">${escapeHtml(copy.sftp.localTitle)}</strong>
            <select id="local-root-sel" class="select mono" style="width:72px;padding:3px 6px;">
              ${this.localRoots.map((r) => `<option value="${escapeHtml(r)}" ${this.localPath.startsWith(r) ? "selected" : ""}>${escapeHtml(r)}</option>`).join("")}
            </select>
            <input id="local-path-input" class="input mono" style="padding:3px 8px;flex:1;" value="${escapeHtml(this.localPath)}" />
            <button id="local-up" class="btn btn-sm">${escapeHtml(copy.sftp.upDir)}</button>
            <button id="local-mkdir" class="btn btn-sm">${escapeHtml(copy.sftp.newFolder)}</button>
            <button id="local-upload-pick" class="btn btn-primary btn-sm">${escapeHtml(copy.sftp.uploadFiles)}</button>
          </div>
          <div class="xftp-table-wrap">
            <table class="xftp-table">
              <thead>
                <tr>
                  <th>${escapeHtml(copy.sftp.colName)}</th>
                  <th>${escapeHtml(copy.sftp.colSize)}</th>
                  <th>${escapeHtml(copy.sftp.colActions)}</th>
                </tr>
              </thead>
              <tbody>
                ${this.localEntries
                  .map(
                    (e) => `
                  <tr data-local-entry="${escapeHtml(e.path)}" data-is-dir="${e.isDir}">
                    <td class="mono" style="cursor:pointer;">${e.isDir ? "[DIR] " : ""}${escapeHtml(e.name)}</td>
                    <td class="mono">${e.isDir ? "-" : escapeHtml(formatBytes(e.size))}</td>
                    <td>
                      ${!e.isDir ? `<button class="btn btn-primary btn-sm" data-upload-file="${escapeHtml(e.path)}">${escapeHtml(copy.sftp.uploadSelected)} &rarr;</button>` : ""}
                    </td>
                  </tr>`
                  )
                  .join("")}
              </tbody>
            </table>
          </div>
        </div>

        <div class="xftp-pane">
          <div class="xftp-toolbar">
            <strong style="font-size:12px;">${escapeHtml(copy.sftp.remoteTitle)}</strong>
            <input id="remote-path-input" class="input mono" style="padding:3px 8px;flex:1;" value="${escapeHtml(this.remotePath)}" />
            <button id="remote-up" class="btn btn-sm">${escapeHtml(copy.sftp.upDir)}</button>
            <button id="remote-mkdir" class="btn btn-sm">${escapeHtml(copy.sftp.newFolder)}</button>
            <button id="remote-refresh" class="btn btn-sm">${escapeHtml(copy.common.refresh)}</button>
          </div>
          <div class="xftp-table-wrap">
            <table class="xftp-table">
              <thead>
                <tr>
                  <th>${escapeHtml(copy.sftp.colName)}</th>
                  <th>${escapeHtml(copy.sftp.colSize)}</th>
                  <th>${escapeHtml(copy.sftp.colActions)}</th>
                </tr>
              </thead>
              <tbody>
                ${this.remoteEntries
                  .map(
                    (e) => `
                  <tr data-remote-entry="${escapeHtml(e.path)}" data-is-dir="${e.isDir}">
                    <td class="mono" style="cursor:pointer;">${e.isDir ? "[DIR] " : ""}${escapeHtml(e.name)}</td>
                    <td class="mono">${e.isDir ? "-" : escapeHtml(formatBytes(e.size))}</td>
                    <td style="display:flex;gap:4px;">
                      ${!e.isDir ? `<button class="btn btn-success btn-sm" data-download-file="${escapeHtml(e.path)}">&larr; ${escapeHtml(copy.sftp.download)}</button>` : ""}
                      <button class="btn btn-sm" data-rename-remote="${escapeHtml(e.path)}">${escapeHtml(copy.sftp.rename)}</button>
                      <button class="btn btn-danger btn-sm" data-del-remote="${escapeHtml(e.path)}" data-dir="${e.isDir}">&times;</button>
                    </td>
                  </tr>`
                  )
                  .join("")}
              </tbody>
            </table>
          </div>
        </div>
      </div>
      <div id="xftp-queue" class="xftp-queue"></div>
    `;

    this.bindEvents();
    this.renderQueue();
  }

  private renderQueue(): void {
    const queueEl = this.container.querySelector<HTMLElement>("#xftp-queue");
    if (!queueEl) return;
    const copy = t();
    const list = Array.from(this.transfers.values()).slice(-8).reverse();
    if (list.length === 0) {
      queueEl.innerHTML = `<div style="color:var(--text-secondary);font-size:11px;">${escapeHtml(copy.sftp.queueTitle)}: ${escapeHtml(copy.sftp.noTransfers)}</div>`;
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
        return `
          <div class="queue-row">
            <span class="badge ${item.state === "completed" ? "badge-dev" : item.state === "failed" || item.state === "cancelled" ? "badge-prod" : "badge-staging"}">${escapeHtml(item.direction.toUpperCase())} ${escapeHtml(item.state)}</span>
            <span class="mono" style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">${escapeHtml(name)}</span>
            <div class="progress-bar"><div class="progress-fill" style="width:${pct}%"></div></div>
            <span class="mono">${pct}%</span>
            ${item.state === "running" ? `<button class="btn btn-danger btn-sm" data-cancel-tx="${escapeHtml(item.id)}">${escapeHtml(copy.common.cancel)}</button>` : ""}
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

  private bindEvents(): void {
    const copy = t();

    this.container.querySelector<HTMLSelectElement>("#local-root-sel")?.addEventListener("change", (e) => {
      this.localPath = (e.target as HTMLSelectElement).value;
      void this.refreshBoth();
    });

    this.container.querySelector<HTMLInputElement>("#local-path-input")?.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        this.localPath = (e.target as HTMLInputElement).value.trim();
        void this.refreshBoth();
      }
    });

    this.container.querySelector("#local-up")?.addEventListener("click", () => {
      const parent = this.localPath.replace(/[\\/][^\\/]+[\\/]?$/, "");
      if (parent) {
        this.localPath = parent.endsWith(":") ? `${parent}\\` : parent;
        void this.refreshBoth();
      }
    });

    this.container.querySelector("#local-mkdir")?.addEventListener("click", async () => {
      const name = window.prompt(copy.sftp.folderNamePrompt);
      if (!name?.trim()) return;
      const sep = this.localPath.endsWith("\\") ? "" : "\\";
      await desktopApi.sftp.makeLocalDir(`${this.localPath}${sep}${name.trim()}`);
      await this.refreshBoth();
    });

    this.container.querySelector("#local-upload-pick")?.addEventListener("click", async () => {
      const files = await desktopApi.sftp.pickLocalFilesToUpload();
      for (const filePath of files ?? []) {
        await desktopApi.sftp.startUpload(this.sessionId, filePath, this.remotePath);
      }
    });

    this.container.querySelectorAll<HTMLElement>("[data-local-entry]").forEach((row) => {
      row.addEventListener("dblclick", () => {
        if (row.dataset.isDir === "true") {
          this.localPath = row.dataset.localEntry ?? this.localPath;
          void this.refreshBoth();
        }
      });
    });

    this.container.querySelectorAll<HTMLButtonElement>("[data-upload-file]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const localFile = btn.dataset.uploadFile ?? "";
        if (localFile) {
          void desktopApi.sftp.startUpload(this.sessionId, localFile, this.remotePath);
        }
      });
    });

    this.container.querySelector<HTMLInputElement>("#remote-path-input")?.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        this.remotePath = (e.target as HTMLInputElement).value.trim() || "/";
        void this.refreshBoth();
      }
    });

    this.container.querySelector("#remote-up")?.addEventListener("click", () => {
      if (this.remotePath === "/") return;
      const idx = this.remotePath.lastIndexOf("/");
      this.remotePath = idx <= 0 ? "/" : this.remotePath.slice(0, idx);
      void this.refreshBoth();
    });

    this.container.querySelector("#remote-mkdir")?.addEventListener("click", async () => {
      const name = window.prompt(copy.sftp.folderNamePrompt);
      if (!name?.trim()) return;
      const base = this.remotePath.endsWith("/") ? this.remotePath : `${this.remotePath}/`;
      await desktopApi.sftp.makeRemoteDir(this.sessionId, `${base}${name.trim()}`);
      await this.refreshBoth();
    });

    this.container.querySelector("#remote-refresh")?.addEventListener("click", () => {
      void this.refreshBoth();
    });

    this.container.querySelectorAll<HTMLElement>("[data-remote-entry]").forEach((row) => {
      row.addEventListener("dblclick", () => {
        if (row.dataset.isDir === "true") {
          this.remotePath = row.dataset.remoteEntry ?? this.remotePath;
          void this.refreshBoth();
        }
      });
    });

    this.container.querySelectorAll<HTMLButtonElement>("[data-download-file]").forEach((btn) => {
      btn.addEventListener("click", () => {
        const remoteFile = btn.dataset.downloadFile ?? "";
        if (remoteFile) {
          void desktopApi.sftp.startDownload(this.sessionId, remoteFile, this.localPath);
        }
      });
    });

    this.container.querySelectorAll<HTMLButtonElement>("[data-rename-remote]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const oldPath = btn.dataset.renameRemote ?? "";
        const newName = window.prompt(copy.sftp.renamePrompt);
        if (!oldPath || !newName?.trim()) return;
        const parent = oldPath.slice(0, oldPath.lastIndexOf("/") + 1);
        await desktopApi.sftp.renameRemote(this.sessionId, oldPath, `${parent}${newName.trim()}`);
        await this.refreshBoth();
      });
    });

    this.container.querySelectorAll<HTMLButtonElement>("[data-del-remote]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const target = btn.dataset.delRemote ?? "";
        const isDir = btn.dataset.dir === "true";
        if (!target || !window.confirm(copy.common.confirmDelete)) return;
        await desktopApi.sftp.deleteRemote(this.sessionId, target, isDir);
        await this.refreshBoth();
      });
    });
  }
}
