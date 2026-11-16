# Arquitectura del Sistema — EC2 Shell Desk

## 1. Visión General

**EC2 Shell Desk** es una aplicación de escritorio nativa para Windows construida con **Go** y **Wails v2** (WebView2), estructurada bajo **Arquitectura Hexagonal (Puertos y Adaptadores)**.

El núcleo del dominio (`internal/core`) es completamente agnóstico a SQLite, `golang.org/x/crypto/ssh`, `github.com/pkg/sftp`, el sistema de archivos Windows y Wails.

## 2. Capas y Reglas de Dependencia

| Capa | Ruta | Responsabilidad | Dependencias Permitidas |
| :--- | :--- | :--- | :--- |
| **Dominio** | `internal/core/domain` | Entidades (`User`, `SSHKey`, `HostInstance`, `FileEntry`, `TransferTask`) y errores centinela. | Solo librería estándar Go. |
| **Puertos** | `internal/core/ports` | Contratos/interfaces para persistencia, criptografía, SSH, SFTP y filesystem local. | `domain`, `context`, `io`. |
| **Servicios** | `internal/services` | Casos de uso (`AuthService`, `VaultService`, `HostService`, `SessionService`, `TransferService`). | `domain`, `ports`. |
| **Adaptadores** | `internal/adapters/*` | Implementaciones concretas (`sqlite`, `crypto`, `sshclient`, `sftpclient`, `winfs`). | `domain`, `ports`, drivers externos. |
| **App / IPC** | `internal/app` | Controladores/bindings expuestos a Wails v2 y emisión de eventos al frontend. | `domain`, `services`, `wails/v2/pkg/runtime`. |
| **Bootstrap** | `cmd/desktop/main.go` | Composición e inyección de dependencias, ciclo de vida Wails (`OnStartup`, `OnShutdown`). | Todas las capas internas. |
| **Frontend** | `frontend/src` | UI Desktop (TypeScript + Vite + Tailwind + `xterm.js`) consumiendo bindings tipados. | Bindings Wails IPC. |

## 3. Concurrencia y Ciclo de Vida de Sesiones

1. **Múltiples Pestañas SSH Simultáneas:**
   - Cada pestaña abierta crea una sesión identificada por `sessionID` (UUID) almacenada en un registro concurrente protegido por `sync.RWMutex`.
   - Cada sesión posee un `context.Context` cancelable propio derivado del contexto de la aplicación.
   - Una goroutine dedicada lee `stdout`/`stderr` del PTY remoto y emite eventos `terminal:data:<sessionID>` hacia WebView2.
   - Un ticker de *keepalive* envía paquetes periódicos `keepalive@openssh.com` para evitar cortes silenciosos en instancias EC2 detrás de NAT/Security Groups.
2. **Transferencias SFTP en Cola:**
   - `TransferService` gestiona tareas `upload` / `download` con su propio `context.CancelFunc` por `transferID`.
   - El streaming por bloques reporta progreso de forma throttled mediante eventos `sftp:progress` sin bloquear los canales de entrada del terminal PTY.
3. **Cierre Limpio:**
   - Al cerrar una pestaña o cerrar la ventana principal (`OnShutdown`), se cancelan los contextos de sesión/transferencia, se cierran los canales SFTP/SSH y se limpian de memoria las claves descifradas.

## 4. Diagrama de Referencia

Consulta [`docs/diagramas/arquitectura-hexagonal.puml`](diagramas/arquitectura-hexagonal.puml).
