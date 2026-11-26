# Arquitectura del Sistema — EC2 Shell Desk

## 1. Visión General

**EC2 Shell Desk** es una aplicación de escritorio nativa para Windows construida con **Go** y **Wails v2** (WebView2), estructurada bajo **Arquitectura Hexagonal (Puertos y Adaptadores)**.

El núcleo del dominio (`internal/core`) es completamente agnóstico a SQLite, `golang.org/x/crypto/ssh`, `github.com/pkg/sftp`, `net/http`, el sistema de archivos Windows y Wails.

## 2. Capas y Reglas de Dependencia

| Capa | Ruta | Responsabilidad | Dependencias Permitidas |
| :--- | :--- | :--- | :--- |
| **Dominio** | `internal/core/domain` | Entidades puras (`User`, `SSHKey`, `HostGroup`, `HostInstance`, `KnownHost`, `SSHTunnel`, `CommandSnippet`, `PrivateService`, `FileEntry`, `TransferTask`) y errores centinela. | Solo librería estándar Go. |
| **Puertos** | `internal/core/ports` | Contratos/interfaces para repositorios, criptografía, SSH, SFTP, filesystem local y sondeo HTTP (`HealthProber`). | `domain`, `context`, `io`, `time`. |
| **Servicios** | `internal/services` | Casos de uso (`AuthService`, `VaultService`, `HostService`, `SessionService`, `TransferService`, `TunnelService`, `SnippetService`, `ObservabilityService`). | `domain`, `ports`, stdlib sin drivers/UI. |
| **Adaptadores** | `internal/adapters/*` | Implementaciones concretas (`sqlite`, `crypto`, `sshclient`, `sftpclient`, `winfs`, `httpprobe`). | `domain`, `ports`, drivers externos. |
| **App / IPC** | `internal/app` | Controladores/bindings expuestos a Wails v2 (`AuthHandler`, `KeyHandler`, `HostHandler`, `TerminalHandler`, `SFTPHandler`, `TunnelHandler`, `SnippetHandler`, `ObservabilityHandler`) y `EventBridge`. | `domain`, `services`, `wails/v2/pkg/runtime`. |
| **Bootstrap** | `cmd/desktop/bootstrap/bootstrap.go` | Composición e inyección de dependencias, ciclo de vida Wails (`OnStartup`, `OnShutdown`). | Todas las capas internas. |
| **Frontend** | `frontend/src` | UI Desktop en Vanilla TypeScript + Vite + CSS nativo + `xterm.js` consumiendo bindings tipados Wails. | Bindings Wails IPC. |

## 3. Concurrencia y Ciclo de Vida de Sesiones

1. **Múltiples Pestañas SSH Simultáneas:**
   - Cada pestaña abierta crea una sesión identificada por `sessionID` (UUID) almacenada en un registro concurrente protegido por `sync.RWMutex`.
   - Cada sesión deriva un contexto cancelable desacoplado (`context.WithoutCancel`) para leer `stdout`/`stderr` del PTY remoto y emitir eventos `terminal:data:<sessionID>` hacia WebView2.
   - Un ticker de *keepalive* envía paquetes periódicos `keepalive@openssh.com` para evitar cortes silenciosos en instancias EC2 detrás de NAT/Security Groups.
2. **Multiplexación SSH para Túneles y Servicios Privados:**
   - `sshclient.Manager` mantiene un pool de clientes SSH con conteo de referencias indexado por huella SHA-256 de host y credenciales.
   - Múltiples túneles de base de datos y servicios de observabilidad (Grafana, Prometheus, Loki, RDS) hacia una misma EC2 multiplexan canales `direct-tcpip` sobre una sola conexión SSH compartida, revalidando `VerifyHostKey` en cada adquisición y cerrando el cliente al liberarse el último consumidor.
3. **Transferencias SFTP en Cola con Historial Acotado:**
   - `TransferService` gestiona subidas de archivos individuales, subidas recursivas de carpetas (`UploadDir`) y descargas con su propio `context.CancelFunc` por `transferID`.
   - El streaming por bloques (64 KB) reporta progreso de forma throttled mediante eventos `sftp:progress` y poda automáticamente tareas finalizadas para acotar la memoria retenida (100 en backend, 50 en frontend).
4. **Cierre Limpio:**
   - Al bloquear la bóveda, cambiar de usuario o cerrar la ventana principal (`OnShutdown`), se cancelan transferencias, túneles, servicios de observabilidad y sesiones PTY, se cierran canales SFTP/SSH y se limpian de memoria las claves derivadas.

## 4. Diagrama de Referencia

Consulta [`docs/diagramas/arquitectura-hexagonal.puml`](diagramas/arquitectura-hexagonal.puml).
