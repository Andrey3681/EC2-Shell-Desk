import * as AuthHandler from "../../wailsjs/go/app/AuthHandler";
import * as HostHandler from "../../wailsjs/go/app/HostHandler";
import * as KeyHandler from "../../wailsjs/go/app/KeyHandler";
import * as SFTPHandler from "../../wailsjs/go/app/SFTPHandler";
import * as TerminalHandler from "../../wailsjs/go/app/TerminalHandler";
import * as TunnelHandler from "../../wailsjs/go/app/TunnelHandler";
import { app, domain, services } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";

export type AuthStatus = app.AuthStatus;
export type UserProfile = domain.UserProfile;
export type SSHKeySummary = domain.SSHKeySummary;
export type HostGroup = domain.HostGroup;
export type HostInstance = domain.HostInstance;
export type KnownHost = domain.KnownHost;
export type SessionInfo = domain.SessionInfo;
export type FileEntry = domain.FileEntry;
export type TransferTask = domain.TransferTask;
export type SSHTunnel = domain.SSHTunnel;
export type TunnelRuntimeStatus = domain.TunnelRuntimeStatus;
export type SaveHostInput = services.SaveHostInput;
export type SaveTunnelInput = services.SaveTunnelInput;

export interface TOFUChallengeEvent {
  requestId: string;
  hostAddress: string;
  port: number;
  keyType: string;
  fingerprint: string;
}

export interface TerminalClosedEvent {
  sessionId: string;
  reason: string;
}

export function decodeBase64ToBytes(encoded: string): Uint8Array {
  const binary = atob(encoded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

export const desktopApi = {
  auth: {
    status: () => AuthHandler.Status(),
    listUsers: async () => (await AuthHandler.ListUsers()) ?? [],
    bootstrapInitialUser: (username: string, password: string) =>
      AuthHandler.BootstrapInitialUser(username, password),
    createUser: (username: string, password: string) =>
      AuthHandler.CreateUser(username, password),
    login: (username: string, password: string) => AuthHandler.Login(username, password),
    lock: () => AuthHandler.Lock(),
  },
  keys: {
    pickFile: () => KeyHandler.PickKeyFile(),
    importFromFile: (name: string, filePath: string, passphrase: string) =>
      KeyHandler.ImportKeyFromFile(name, filePath, passphrase),
    importContent: (name: string, rawPEM: string, passphrase: string) =>
      KeyHandler.ImportKeyContent(name, rawPEM, passphrase),
    list: async () => (await KeyHandler.ListKeys()) ?? [],
    delete: (keyId: string) => KeyHandler.DeleteKey(keyId),
  },
  hosts: {
    list: async () => (await HostHandler.ListHosts()) ?? [],
    create: (input: SaveHostInput) => HostHandler.CreateHost(input),
    update: (input: SaveHostInput) => HostHandler.UpdateHost(input),
    delete: (hostId: string) => HostHandler.DeleteHost(hostId),
  },
  groups: {
    list: async () => (await HostHandler.ListGroups()) ?? [],
    create: (name: string) => HostHandler.CreateGroup(name),
    rename: (groupId: string, name: string) => HostHandler.RenameGroup(groupId, name),
    delete: (groupId: string) => HostHandler.DeleteGroup(groupId),
  },
  terminal: {
    openSession: (hostId: string, cols: number, rows: number, passwordFallback: string) =>
      TerminalHandler.OpenSession(hostId, cols, rows, passwordFallback),
    respondTOFU: (requestId: string, trusted: boolean) =>
      TerminalHandler.RespondTOFU(requestId, trusted),
    write: (sessionId: string, data: string) => TerminalHandler.WriteTerminal(sessionId, data),
    resize: (sessionId: string, cols: number, rows: number) =>
      TerminalHandler.ResizeTerminal(sessionId, cols, rows),
    closeSession: (sessionId: string) => TerminalHandler.CloseSession(sessionId),
    listSessions: async () => (await TerminalHandler.ListSessions()) ?? [],
    listKnownHosts: async () => (await TerminalHandler.ListKnownHosts()) ?? [],
    removeKnownHost: (knownHostId: string) => TerminalHandler.RemoveKnownHost(knownHostId),
    onData: (sessionId: string, cb: (bytes: Uint8Array) => void) =>
      EventsOn(`terminal:data:${sessionId}`, (encoded: string) => {
        if (typeof encoded === "string" && encoded.length > 0) {
          cb(decodeBase64ToBytes(encoded));
        }
      }),
    onClosed: (sessionId: string, cb: (ev: TerminalClosedEvent) => void) =>
      EventsOn(`terminal:closed:${sessionId}`, cb),
    onTOFUPrompt: (cb: (ev: TOFUChallengeEvent) => void) => EventsOn("ssh:tofu-prompt", cb),
  },
  sftp: {
    localHomeDir: () => SFTPHandler.LocalHomeDir(),
    listLocalRoots: async () => (await SFTPHandler.ListLocalRoots()) ?? [],
    listLocalDir: async (path: string) => (await SFTPHandler.ListLocalDir(path)) ?? [],
    makeLocalDir: (path: string) => SFTPHandler.MakeLocalDir(path),
    renameLocal: (oldPath: string, newPath: string) => SFTPHandler.RenameLocal(oldPath, newPath),
    deleteLocal: (path: string) => SFTPHandler.DeleteLocal(path),
    pickLocalFilesToUpload: async () => (await SFTPHandler.PickLocalFilesToUpload()) ?? [],
    pickLocalDirectory: () => SFTPHandler.PickLocalDirectory(),
    remoteWorkingDir: (sessionId: string) => SFTPHandler.RemoteWorkingDir(sessionId),
    listRemoteDir: async (sessionId: string, path: string) =>
      (await SFTPHandler.ListRemoteDir(sessionId, path)) ?? [],
    makeRemoteDir: (sessionId: string, path: string) =>
      SFTPHandler.MakeRemoteDir(sessionId, path),
    renameRemote: (sessionId: string, oldPath: string, newPath: string) =>
      SFTPHandler.RenameRemote(sessionId, oldPath, newPath),
    deleteRemote: (sessionId: string, path: string, recursive: boolean) =>
      SFTPHandler.DeleteRemote(sessionId, path, recursive),
    startUpload: (sessionId: string, localFilePath: string, remoteDirPath: string) =>
      SFTPHandler.StartUpload(sessionId, localFilePath, remoteDirPath),
    startDownload: (sessionId: string, remoteFilePath: string, localDirPath: string) =>
      SFTPHandler.StartDownload(sessionId, remoteFilePath, localDirPath),
    cancelTransfer: (transferId: string) => SFTPHandler.CancelTransfer(transferId),
    listTransfers: async () => (await SFTPHandler.ListTransfers()) ?? [],
    onProgress: (cb: (task: TransferTask) => void) => EventsOn("sftp:progress", cb),
  },
  tunnels: {
    list: async () => (await TunnelHandler.ListTunnels()) ?? [],
    create: (input: SaveTunnelInput) => TunnelHandler.CreateTunnel(input),
    update: (input: SaveTunnelInput) => TunnelHandler.UpdateTunnel(input),
    delete: (tunnelId: string) => TunnelHandler.DeleteTunnel(tunnelId),
    start: (tunnelId: string, passwordFallback: string) =>
      TunnelHandler.StartTunnel(tunnelId, passwordFallback),
    stop: (tunnelId: string) => TunnelHandler.StopTunnel(tunnelId),
    listStatuses: async () => (await TunnelHandler.ListStatuses()) ?? [],
    onStatus: (cb: (status: TunnelRuntimeStatus) => void) => EventsOn("tunnel:status", cb),
  },
};
