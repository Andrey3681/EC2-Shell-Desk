# Guía Maestra de Contextualización, Estructura, Skills y Herramientas — Cliente Desktop Windows SSH + SFTP en Go (Estilo Xshell + Xftp)

Este documento contextualiza la arquitectura, distribución de código, **Skills para Agentes de IA**, reglas de gobernanza (`AGENTS.md`), estrategia de documentación (`README.md`, `docs/`, `gomarkdoc`), estándar de commits y herramientas para construir una **aplicación de escritorio para Windows en Go** inspirada en **Xshell + Xftp**, enfocada exclusivamente en simplificar la conexión SSH y la transferencia de archivos/documentos hacia **instancias EC2 y servidores Linux**.

> [!NOTE]
> **Estado del Documento:** Este archivo es exclusivamente una **guía de contextualización y estructuración inicial**. No se debe desarrollar código fuente todavía; funciona como plano maestro de referencia antes de inicializar el repositorio.

---

## 1. Contexto del Proyecto y Alcance Funcional

### 1.1 Objetivo del Sistema
Construir una aplicación de escritorio nativa y liviana para **Windows (`.exe` / instalador NSIS)** que combine en una sola interfaz práctica:
1. **Cliente de Terminal SSH (estilo Xshell simplificado):** Conexión rápida en 1 clic a instancias EC2 sin depender de comandos manuales en consola (`ssh -i ...`).
2. **Gestor de Archivos SFTP Integrado (estilo Xftp simplificado):** Explorador de archivos acoplado (Local Windows ↔ Remoto EC2) para subir, descargar y administrar documentos fácilmente mediante botones o *drag & drop*.
3. **Aislamiento Multi-Usuario desde el Arranque:** Incluye un **usuario inicial** (perfil administrador por defecto o asistente inicial) y soporte para múltiples usuarios locales; **cada usuario tiene sus propias instancias/máquinas virtuales y sus propias llaves SSH aisladas**.
4. **Bóveda Automatizada de Llaves SSH:** Permite subir/importar llaves privadas (`.pem`, `id_ed25519`, `id_rsa` y opcionalmente `.ppk`) cifradas en reposo y vincularlas a cada instancia para automatizar tanto la terminal SSH como el panel SFTP.

> [!IMPORTANT]
> **Enfoque Minimalista ("Sin Bloat"):** A diferencia de Xshell completo, **no** se implementan protocolos heredados ni complejos (Telnet, Serial/COM, Rlogin, X11 forwarding, scripts VBScript/JScript). Todo el diseño gira en torno a **Usuario → Llaves SSH → Instancias EC2 → Terminal + SFTP**.

### 1.2 Arquitectura Tecnológica (Go + Wails + xterm.js)

Para una aplicación de escritorio en Windows escrita en **Go** que requiere tanto una UI moderna como un emulador de terminal ANSI/VT100 fiel (colores, `vim`, `htop`, `top`, `less`, redimensionamiento PTY, copiar/pegar) y un gestor de archivos dual-pane, la combinación recomendada es:

```text
+-----------------------------------------------------------------------------------+
|                     Aplicación de Escritorio Windows (Wails v2)                   |
|  +-----------------------------------------------------------------------------+  |
|  |         Frontend Desktop (WebView2 + TypeScript + xterm.js + UI Xftp)       |  |
|  |  - Login / Selector de Usuario Inicial                                      |  |
|  |  - Panel Lateral: Mis Instancias EC2 & Bóveda de Llaves (.pem)              |  |
|  |  - Pestañas Centrales: Terminales SSH (xterm.js)                            |  |
|  |  - Panel Integrado / Split View: Gestor de Archivos Local <-> Remoto (Xftp) |  |
|  +--------------------------------------+--------------------------------------+  |
|                                         | Wails IPC / Bindings + Stream WS/Events |
|  +--------------------------------------v--------------------------------------+  |
|  |                  Núcleo Backend en Go (Arquitectura Hexagonal)              |  |
|  |  - Auth & Multi-User Isolation (Argon2id + AES-256-GCM / Windows DPAPI)     |  |
|  |  - SSH Session Manager (golang.org/x/crypto/ssh + PTY + Keepalive)          |  |
|  |  - SFTP Transfer Engine (github.com/pkg/sftp + cola con progreso/cancel)    |  |
|  |  - Vault Repository (SQLite local modernc.org/sqlite sin CGO)               |  |
|  +--------------------------------------+--------------------------------------+  |
+-----------------------------------------|-----------------------------------------+
                                          | SSH / SFTP (TCP :22 / Custom Port)
                                          v
                        +-----------------------------------+
                        |   Instancias EC2 / VMs Linux      |
                        |  (ec2-user / ubuntu / admin ...)  |
                        +-----------------------------------+
```

### 1.3 Casos de Uso Fundamentales
1. **Usuario Inicial y Perfiles Aislados:**
   - Al iniciar por primera vez, existe un **usuario inicial** configurable (o creación guiada en primer arranque).
   - Cada usuario inicia sesión con su contraseña local (derivada con **Argon2id** para desbloquear su bóveda cifrada).
   - Todas las consultas de instancias, llaves y sesiones recientes se filtran estrictamente por `user_id`.
2. **Gestión y Carga de Llaves SSH:**
   - El usuario sube su archivo `.pem` (o clave OpenSSH) desde Windows, le asigna un nombre legible (ej. `aws-prod-us-east-1.pem`) y opcionalmente guarda su *passphrase*.
   - La llave se cifra con **AES-256-GCM** antes de persistirse en la bóveda local del usuario (nunca queda una copia temporal sin limpiar o en texto plano en la carpeta del proyecto).
3. **Configuración de Instancias EC2 / VMs:**
   - Formulario claro con:
     - **Alias / Nombre:** ej. `Backend API Prod`
     - **Dirección IP o Hostname DNS:** ej. `54.123.45.67` o `ec2-54-123-45-67.compute-1.amazonaws.com`
     - **Puerto SSH:** `22` por defecto
     - **Usuario SSH:** `ec2-user`, `ubuntu`, `admin`, `debian`, `rocky`, etc.
     - **Llave SSH Asociada:** selector de las llaves cargadas por el usuario actual (o autenticación por contraseña como respaldo)
     - **Etiqueta / Entorno:** `Prod`, `Staging`, `Dev` y notas rápidas
4. **Conexión SSH Automática (Modo Xshell):**
   - Doble clic o botón **Conectar**: autentica automáticamente usando la llave asignada, verifica huella del servidor (*TOFU — Trust On First Use* con confirmación visual en la primera conexión) y abre una pestaña de terminal interactiva.
5. **Transferencia de Documentos Integrada (Modo Xftp):**
   - Desde la misma ventana (panel acoplado o pestaña SFTP de la instancia conectada), el usuario navega sus carpetas locales de Windows y el sistema de archivos remoto de la instancia EC2.
   - Permite subir archivos/documentos por selector o *drag & drop*, descargar a Windows, renombrar, crear directorios, eliminar y visualizar barra de progreso por transferencia.

---

## 2. Distribución del Código y Estructura del Repositorio

Se aplica **Arquitectura Hexagonal (Puertos y Adaptadores)** en Go, desacoplando por completo la lógica de negocio (usuarios, bóveda de llaves, instancias, sesiones) de `x/crypto/ssh`, `pkg/sftp`, SQLite y Wails.

### 2.1 Árbol de Directorios del Repositorio

```text
ec2-shell-desk/
├── .agents/                               # Gobernanza, reglas y skills para Agentes de IA
│   ├── rules/                             # Reglas modulares siempre activas
│   │   ├── go-hexagonal-architecture.md   # Reglas de capas core/ports/adapters/services
│   │   ├── windows-desktop-wails.md       # Convenciones Wails v2, IPC, WebView2 y xterm.js
│   │   ├── ssh-sftp-vault-security.md     # Reglas de cifrado de llaves, PTY, SFTP y known_hosts
│   │   └── code-english-and-commits.md    # Atributos/comentarios en inglés y Conventional Commits
│   └── skills/                            # Habilidades bajo demanda (SKILL.md + references/)
│       ├── wails-desktop/                 # [Ref: go-skills/wails] Patrones Wails v2 y bindings TS
│       ├── golang-engineering/            # [Ref: affaan-m/ecc + samber/cc-skills-golang] Go idiomático
│       ├── go-testing/                    # [Ref: affaan-m/ecc] TDD, table-driven tests y fakes
│       ├── security-auth/                 # [Ref: ajianaz/skills-collection] Auth local, Argon2id, AES-GCM
│       ├── database-manager/              # [Ref: ajianaz/skills-collection] SQLite y migraciones
│       ├── qa-tester/                     # [Ref: ajianaz/skills-collection] Plan QA, pruebas SSH/SFTP
│       ├── agents-md-generator/           # [Ref: ajianaz/skills-collection] Mantenimiento de AGENTS.md
│       ├── go-code-documentation/         # [Proyecto + gomarkdoc] Docstrings Go en inglés -> docs/api
│       ├── conventional-commits/          # [Ref: github/awesome-copilot] Commits semánticos Git
│       ├── readme-and-docs/               # [Ref: github/awesome-copilot] README, docs/ y CHANGELOG
│       └── ssh-sftp-pty-engine/           # [Proyecto] Motor PTY xterm.js + SFTP dual-pane estilo Xftp
├── .github/
│   └── workflows/                         # CI Windows (lint, test, build .exe + NSIS installer)
├── build/                                 # Recursos de empaquetado Windows (Wails icon, manifest, NSIS)
│   └── windows/
│       ├── icon.ico
│       ├── info.json
│       └── installer/
├── cmd/
│   └── desktop/
│       └── main.go                        # Punto de entrada Wails, DI y arranque del usuario inicial
├── docs/                                  # Documentación técnica, funcional y de seguridad
│   ├── api/                               # Documentación Go autogenerada con gomarkdoc
│   ├── diagramas/                         # Diagramas PlantUML (.puml) y exportados (.png)
│   │   ├── arquitectura-hexagonal.puml
│   │   ├── flujo-boveda-llaves.puml
│   │   └── flujo-ssh-sftp.puml
│   ├── arquitectura.md                    # Puertos, adaptadores, reglas de dependencia y concurrencia
│   ├── diseno-solucion.md                 # Flujos de usuario inicial, multi-usuario, SSH y SFTP
│   ├── seguridad-boveda.md                # Modelo criptográfico (Argon2id, AES-256-GCM, known_hosts)
│   ├── ui-ux-spec.md                      # Especificación de pantallas (Login, Terminal Xshell, SFTP Xftp)
│   ├── verificacion.md                    # Matriz de pruebas manuales y automatizadas (EC2 / Docker SSH)
│   └── TODO.md                            # Lista de verificación de fases de desarrollo
├── frontend/                              # UI de escritorio (Vite + TypeScript + Tailwind + xterm.js)
│   ├── src/
│   │   ├── components/                    # Sidebar instancias, KeyVaultModal, XftpDualPane, TerminalTabs
│   │   ├── services/                      # Wrappers tipados sobre bindings auto-generados de Wails
│   │   ├── i18n/                          # Textos de interfaz (es / en)
│   │   └── main.ts
│   ├── package.json
│   └── tsconfig.json
├── internal/
│   ├── app/                               # Controladores/Bindings expuestos a la UI de Wails
│   │   ├── auth_handler.go                # Login, creación de usuarios, bloqueo de bóveda
│   │   ├── host_handler.go                # CRUD de instancias EC2 por usuario
│   │   ├── key_handler.go                 # Importación/listado/eliminación de llaves .pem
│   │   ├── terminal_handler.go            # Apertura, escritura, resize y cierre de pestañas SSH
│   │   └── sftp_handler.go                # Listado remoto/local, upload, download y cancelación
│   ├── core/                              # Núcleo puro de dominio (sin dependencias externas)
│   │   ├── domain/                        # Entidades e invariantes de negocio
│   │   │   ├── user.go                    # User (ID, Username, PasswordHash, Salt, IsDefault, CreatedAt)
│   │   │   ├── ssh_key.go                 # SSHKey (ID, UserID, Name, KeyType, Fingerprint, EncryptedPEM)
│   │   │   ├── host_instance.go           # HostInstance (ID, UserID, Alias, HostAddress, Port, SSHUser, KeyID)
│   │   │   ├── file_entry.go              # FileEntry (Name, Path, Size, IsDir, Permissions, ModifiedAt)
│   │   │   ├── transfer_task.go           # TransferTask (ID, Direction, LocalPath, RemotePath, Progress)
│   │   │   └── errors.go                  # Errores centinela del dominio
│   │   └── ports/                         # Interfaces que definen contratos del núcleo
│   │       ├── repository.go              # UserRepository, HostRepository, KeyRepository
│   │       ├── crypto.go                  # KeyEncryptor & PasswordHasher
│   │       ├── ssh_engine.go              # SSHSessionManager (Connect, OpenPTY, Resize, Close)
│   │       ├── sftp_engine.go             # SFTPClient (ListDir, Upload, Download, Mkdir, Remove, Rename)
│   │       └── local_fs.go                # LocalFileSystem (navegación segura en disco de Windows)
│   ├── adapters/                          # Implementaciones concretas de los puertos
│   │   ├── sqlite/                        # Persistencia SQLite con migraciones embebidas y filtro user_id
│   │   ├── crypto/                        # Argon2id + AES-256-GCM para llaves privadas en reposo
│   │   ├── sshclient/                     # Adaptador golang.org/x/crypto/ssh + parser .pem/.ppk + known_hosts
│   │   ├── sftpclient/                    # Adaptador github.com/pkg/sftp con streaming y progreso
│   │   └── winfs/                         # Adaptador de sistema de archivos Windows (unidades C:\, D:\, etc.)
│   └── services/                          # Casos de uso de la aplicación
│       ├── auth_service.go                # Bootstrap de usuario inicial, login y desbloqueo de clave maestra
│       ├── vault_service.go               # Alta/lectura/cifrado de llaves SSH por usuario
│       ├── host_service.go                # Gestión de instancias EC2 aisladas por usuario
│       ├── session_service.go             # Orquestación de conexión SSH + apertura de canal SFTP
│       └── transfer_service.go            # Cola de subidas/descargas Xftp con eventos de progreso
├── scripts/                               # Scripts utilitarios (Windows PowerShell)
│   ├── dev-doctor.ps1                     # Verifica Go, Wails CLI, Node.js, WebView2, NSIS, linters
│   └── verify-commit-msg.ps1              # Valida formato Conventional Commits en hook local
├── testdata/                              # Fixtures de prueba (llaves dummy generadas para test, nunca reales)
├── .gitignore
├── .golangci.yml                          # Reglas estrictas de calidad estática en Go
├── .gomarkdoc.yml                         # Generación automática de docs/api/
├── AGENTS.md                              # Reglas maestras de comportamiento para IA
├── CHANGELOG.md                           # Historial de versiones (Keep a Changelog)
├── CONTRIBUTING.md                        # Estándar de ramas, PRs, commits y documentación
├── estructura copy.md                     # Este documento maestro de contextualización
├── go.mod
├── go.sum
├── lefthook.yml                           # Git hooks locales (commitlint, gofumpt, lint, test)
├── README.md                              # Guía rápida de instalación, uso y compilación Windows
├── Taskfile.yml                           # Orquestador unificado del ciclo de desarrollo
└── wails.json                             # Configuración de build/dev de la app de escritorio Wails
```

---

## 3. Catálogo de Skills para Agentes de IA (`.agents/skills/`)

Se combinan skills verificadas de **`ajianaz/skills-collection`**, skills especializadas del ecosistema **Go / Wails**, skills para **documentación de código, commits y README**, y skills propias del dominio **SSH / SFTP**.

### 3.1 Skills Base desde `ajianaz/skills-collection`
Repositorio: `https://github.com/ajianaz/skills-collection/tree/main/skills`

| Skill | Origen en `ajianaz/skills-collection` | Aplicación en este Proyecto Desktop |
| :--- | :--- | :--- |
| **`security-auth`** | `skills/security-auth` | Diseño de autenticación local multi-usuario, hashing Argon2id, derivación de clave maestra de sesión para descifrar llaves `.pem` en memoria y aislamiento estricto por `user_id`. |
| **`database-manager`** | `skills/database-manager` | Modelado relacional en SQLite (`users`, `ssh_keys`, `host_instances`, `known_hosts`), migraciones versionadas, índices y respaldo seguro de la base local. |
| **`qa-tester`** | `skills/qa-tester` | Diseño de matriz de pruebas para conexiones EC2 reales/simuladas (servidor SSH embebido en tests), caídas de red, permisos de archivos y aislamiento entre usuarios. |
| **`agents-md-generator`** | `skills/agents-md-generator` | Mantenimiento del archivo `AGENTS.md` y mapas de contexto a medida que crecen los adaptadores y la UI. |
| **`task-generator`** | `skills/task-generator` | Desglose de funcionalidades (bóveda, terminal, SFTP, instalador Windows) en subtareas ordenadas y verificables. |
| **`observability-monitor`** | `skills/observability-monitor` | Logging estructurado (`log/slog`) de sesiones y transferencias **prohibiendo registrar** llaves privadas, passphrases o contraseñas en logs. |

### 3.2 Skills de Go, Wails Desktop, Documentación y Commits

| Skill | Referencia / Origen | Propósito y Directrices Clave |
| :--- | :--- | :--- |
| **`wails-desktop`** | `go-skills/wails` (`spf13/go-skills`) | Evita mezclar APIs incompatibles entre Wails v2 y v3; guía generación de bindings Go ↔ TypeScript, eventos de streaming (`EventsEmit` para terminal/progreso SFTP), diálogo nativo de selección de archivos `.pem` y empaquetado Windows `.exe`. |
| **`golang-engineering`** | `affaan-m/ecc` (`golang-patterns`) + `cxuu/golang-skills` (`go-code-review`) | Gobernanza de Go idiomático, Arquitectura Hexagonal estricta, manejo de errores con `%w` / `errors.Is`, control de concurrencia (`sync.Mutex`, cancelación con `context.Context`) y limpieza de goroutines al cerrar pestañas. |
| **`go-testing`** | `affaan-m/ecc` (`golang-testing`) | Ciclo TDD, **Table-Driven Tests** con `t.Run`, dobles de prueba en memoria para puertos (`fakeSSHEngine`, `fakeSFTPEngine`) y servidor SSH/SFTP local en memoria para tests de integración sin depender de AWS EC2. |
| **`go-code-documentation`** | Adaptada con `gomarkdoc` + Effective Go | Exige que **todos los identificadores, atributos de structs y comentarios (`//`) estén en inglés**. Toda entidad, interfaz, función y método exportado inicia su comentario con su propio nombre (`// HostInstance represents...`) y se compila hacia `docs/api/` con `gomarkdoc`. |
| **`conventional-commits`** | `github/awesome-copilot` (`conventional-commit`) | Impone inspección de `git diff --staged` y mensajes **Conventional Commits 1.0.0** en inglés imperativo (`feat(sftp): add drag and drop upload queue`). |
| **`readme-and-docs`** | `github/awesome-copilot` (`documentation-writer`) + `ajianaz/skills-collection` (`agents-md-generator`) | Mantiene actualizados `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md` y `docs/` (arquitectura, modelo de cifrado de bóveda y diagramas PlantUML). |

### 3.3 Skill Propia del Dominio (`ssh-sftp-pty-engine`)
- **Ubicación:** `.agents/skills/ssh-sftp-pty-engine/SKILL.md`
- **Propósito:** Estandarizar el núcleo técnico estilo **Xshell + Xftp**:
  1. Conexión SSH con `golang.org/x/crypto/ssh`, soporte de llaves RSA, Ed25519 y ECDSA en formato PEM/OpenSSH (y conversión/advertencia clara si el usuario sube un `.ppk` de PuTTY).
  2. Verificación de clave de host (*TOFU* + persistencia de `known_hosts` por usuario) sin usar `ssh.InsecureIgnoreHostKey()` en silencio.
  3. Puente bidireccional entre `xterm.js` y el PTY remoto (manejo de `WindowChange` al redimensionar la ventana de Windows).
  4. Sesión SFTP con `github.com/pkg/sftp`: copia en chunks con `io.Reader` instrumentado para reportar porcentaje/velocidad a la UI y cancelación segura vía `context.Context`.

#### Comandos de referencia para instalar las skills públicas al inicializar el proyecto
```bash
# Skills base desde ajianaz/skills-collection
npx skills add ajianaz/skills-collection --skill security-auth
npx skills add ajianaz/skills-collection --skill database-manager
npx skills add ajianaz/skills-collection --skill qa-tester
npx skills add ajianaz/skills-collection --skill agents-md-generator

# Skills especializadas para Go, Wails Desktop y Conventional Commits
npx skills add https://github.com/go-skills/wails
npx skills add affaan-m/ecc --skill golang-patterns
npx skills add affaan-m/ecc --skill golang-testing
npx skills add github/awesome-copilot --skill conventional-commit
npx skills add github/awesome-copilot --skill documentation-writer
```

---

## 4. Gobernanza Obligatoria en `AGENTS.md`

El archivo `AGENTS.md` en la raíz del proyecto define las reglas inquebrantables para cualquier agente de IA. **Establece explícitamente que todos los identificadores, atributos y comentarios de código estén en inglés.**

### Plantilla Mandatoria de `AGENTS.md` para el Futuro Proyecto

```markdown
# Engineering & AI Agent Rules — EC2 Shell Desk (Windows Go + Wails)

You are an expert in Go, Wails v2 desktop development for Windows, SSH/SFTP protocols, local cryptography, and Hexagonal Architecture.

## 1. Language Rules (STRICT)
- **Code Identifiers & Struct Attributes in English:** ALL Go packages, file names, interfaces, structs, struct fields/attributes, functions, methods, variables, constants, SQL columns, and TypeScript types/props MUST be written in **English** (e.g., `HostInstance`, `hostAddress`, `sshUsername`, `privateKeyPEM`, `ownerUserID`).
- **Code Comments & Docstrings in English:** ALL inline comments (`//`), TODOs, package docs, and exported Go doc comments (`// SymbolName ...`) MUST be written in **English**.
- **Sensitive Logging Forbidden:** NEVER log private keys, passphrases, user passwords, or decrypted secrets in logs, errors, or test snapshots.
- **UI Copy via i18n:** Keep user-facing UI strings isolated in `frontend/src/i18n/` (`es` / `en`), decoupled from Go domain logic.

## 2. Hexagonal Architecture & Separation of Concerns
- Follow strict Hexagonal Architecture: `internal/core/{domain,ports}`, `internal/adapters`, `internal/services`, `internal/app`, and `frontend/`.
- `internal/core/` MUST NOT import concrete adapters (`sqlite`, `sshclient`, `sftpclient`), Wails runtime, or UI frameworks.
- Dependency injection happens exclusively in `cmd/desktop/main.go`.
- Every database query for instances, SSH keys, or known hosts MUST enforce `user_id` ownership isolation.

## 3. Security & SSH/SFTP Standards
- Encrypt all stored SSH private keys at rest per user using Argon2id key derivation + AES-256-GCM (never store plaintext `.pem` files inside app data).
- Wipe sensitive byte slices from memory when sessions lock or keys are unloaded where practical.
- Never silently ignore host key verification (`InsecureIgnoreHostKey` is forbidden in production paths; implement TOFU prompt and persist verified host fingerprints).
- Always attach timeouts and `context.Context` cancellation to SSH dialing, PTY sessions, and SFTP file transfers so closing a tab never leaks goroutines.

## 4. Documentation, Verification & Commits
- Every exported Go type, function, method, and constant MUST have a valid Go doc comment in English starting with its exact name (compatible with `gomarkdoc`).
- Keep `README.md`, `CHANGELOG.md`, and `docs/` synchronized whenever features, schema, or build steps change.
- Run `task check` (`gofumpt`, `golangci-lint`, `go test`) before considering any task complete.
- Follow **Conventional Commits 1.0.0** in English imperative mood (`feat(vault): encrypt imported PEM keys with AES-256-GCM`).
```

---

## 5. Estrategia de Documentación (`README.md`, `docs/` y `gomarkdoc`)

### 5.1 `README.md` (Raíz del Repositorio)
Debe permitir que cualquier desarrollador entienda, configure y compile el `.exe` en Windows rápidamente:
1. **Visión general:** Qué resuelve la app (alternativa enfocada y gratuita estilo Xshell + Xftp para instancias EC2).
2. **Funcionalidades principales:** Usuario inicial y multi-perfil, bóveda de llaves `.pem`, terminal SSH con pestañas, gestor SFTP dual-pane integrado.
3. **Modelo de seguridad resumido:** Cifrado local de llaves por usuario, sin telemetría ni envío de llaves a la nube.
4. **Requisitos en Windows:** Go 1.24+, Wails CLI v2, Node.js LTS, WebView2 Runtime y NSIS (solo para generar el instalador).
5. **Comandos rápidos (`Taskfile.yml`):** `task doctor`, `task dev`, `task check`, `task doc`, `task build-windows`.
6. **Arquitectura resumida y enlaces a `docs/`.**

### 5.2 Documentos en `docs/`
- **`docs/arquitectura.md`**: Límites hexagonales, diseño de concurrencia de sesiones SSH/SFTP simultáneas y comunicación Go ↔ Frontend (Wails bindings + eventos).
- **`docs/seguridad-boveda.md`**: Flujo de arranque del **usuario inicial**, derivación de clave maestra con Argon2id, esquema de cifrado AES-256-GCM para llaves `.pem`, almacenamiento de `known_hosts` y recomendaciones de permisos en Windows.
- **`docs/diseno-solucion.md`**: Casos de uso detallados (alta de usuario, carga de llaves, creación de instancia EC2 con IP/puerto/usuario, conexión SSH, subida/descarga SFTP).
- **`docs/ui-ux-spec.md`**: Distribución visual inspirada en Xshell + Xftp (barra de sesiones por usuario, pestañas de terminal, panel inferior/lateral SFTP dual-pane y cola de transferencias).
- **`docs/verificacion.md`**: Protocolo de pruebas unitarias, integración contra contenedor/servidor SSH local de prueba y pruebas manuales en Windows contra una instancia EC2 real.
- **`docs/diagramas/`**: Archivos `.puml` (arquitectura hexagonal, flujo criptográfico de bóveda, flujo SSH+SFTP) compilados a `.png`.
- **`docs/api/`**: Referencia técnica en Markdown generada automáticamente desde los docstrings en inglés mediante **`gomarkdoc`**.

### 5.3 Estándar de Documentación de Código en Go (`gomarkdoc`)
Ejemplo del estándar obligatorio en inglés:

```go
// HostInstance represents a remote virtual machine or EC2 instance owned by a local user.
type HostInstance struct {
	ID          string
	OwnerUserID string
	Alias       string
	HostAddress string
	Port        int
	SSHUsername string
	SSHKeyID    string
	Environment string
}
```

---

## 6. Estándar de Commits y Flujo Git

Se aplica **Conventional Commits 1.0.0** en **inglés** y **modo imperativo**:

```text
<type>(<scope>): <short imperative summary>

[optional body explaining why]

[optional footer: Refs #id | BREAKING CHANGE: ...]
```

- **Tipos:** `feat`, `fix`, `docs`, `refactor`, `test`, `perf`, `style`, `build`, `ci`, `chore`, `revert`.
- **Scopes sugeridos:** `auth`, `vault`, `hosts`, `ssh`, `sftp`, `terminal`, `ui`, `sqlite`, `crypto`, `windows`, `docs`, `ci`.
- **Ejemplos válidos:**
  - `feat(auth): bootstrap initial local user on first application launch`
  - `feat(vault): support encrypted PEM key import per user`
  - `feat(sftp): add dual-pane remote directory listing and file upload`
  - `docs(security): document Argon2id and AES-256-GCM vault key derivation`
- **Validación automática:** Hook local con `lefthook.yml` validando el formato del commit, `gofumpt`, `golangci-lint` y `go test`.

---

## 7. Herramientas, Calidad y Automatización (`Taskfile.yml`)

### 7.1 Herramientas de Desarrollo y Distribución en Windows

| Herramienta / Paquete | Rol en el Proyecto |
| :--- | :--- |
| **Go 1.24+** | Lenguaje principal del núcleo hexagonal, SSH, SFTP y criptografía. |
| **Wails CLI v2** | Empaquetado nativo de escritorio Windows usando Microsoft Edge **WebView2** (binario ligero sin incluir Chromium). |
| **Frontend (`xterm.js` + Vite + TS)** | Emulador de terminal VT100/ANSI completo + explorador dual-pane estilo Xftp. |
| **`golang.org/x/crypto/ssh` + `argon2`** | Cliente SSH, PTY, autenticación por llave pública y derivación Argon2id. |
| **`github.com/pkg/sftp`** | Motor SFTP para listado, subida, descarga, renombrado y gestión remota de archivos. |
| **`modernc.org/sqlite`** | Base de datos SQLite pura en Go (sin necesidad de compilador C/MinGW en Windows). |
| **NSIS (Nullsoft Scriptable Install System)** | Integrado con `wails build -nsis` para generar un instalador estándar de Windows (`setup.exe`). |
| **`gofumpt` + `golangci-lint`** | Formateo estricto y análisis estático de Go (`.golangci.yml`). |
| **`gomarkdoc`** | Generación automática de documentación Markdown en `docs/api/` a partir de docstrings Go. |
| **`PlantUML` + `Task`** | Diagramas de arquitectura como código y automatización unificada vía `Taskfile.yml`. |
| **`Lefthook`** | Ejecución local de Git hooks en Windows (commitlint, formato, lint y pruebas). |

### 7.2 Orquestador de Tareas (`Taskfile.yml`)

```yaml
version: "3"

vars:
  APP_NAME: ec2-shell-desk
  PKGS: ./...

tasks:
  doctor:
    desc: Verificar herramientas en Windows (Go, Wails, Node, WebView2, NSIS, linters)
    cmd: pwsh -File scripts/dev-doctor.ps1

  setup:
    desc: Descargar dependencias Go/Frontend e instalar herramientas de calidad
    cmds:
      - go mod download
      - cmd: npm ci
        dir: frontend
      - go install github.com/wailsapp/wails/v2/cmd/wails@latest
      - go install mvdan.cc/gofumpt@latest
      - go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
      - go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest

  dev:
    desc: Iniciar la aplicación de escritorio en modo desarrollo con hot-reload
    cmd: wails dev

  fmt:
    desc: Formatear código Go con gofumpt estricto
    cmd: gofumpt -l -w .

  lint:
    desc: Ejecutar análisis estático de Go con golangci-lint
    cmd: golangci-lint run {{.PKGS}}

  test:
    desc: Ejecutar suite de pruebas unitarias e integración en Go con detector de carreras
    cmds:
      - task: fmt
      - go test {{.PKGS}} -race -cover -count=1

  doc:
    desc: Regenerar documentación Markdown de Go en docs/api y diagramas PlantUML
    cmds:
      - gomarkdoc -o "docs/api/{{.Dir}}.md" ./internal/...
      - plantuml -tpng docs/diagramas/*.puml

  check:
    desc: Validación integral previa a commit (formato + lint + tests + doc)
    cmds:
      - task: fmt
      - task: lint
      - task: test
      - task: doc

  build:
    desc: Compilar ejecutable portátil de Windows (.exe)
    cmd: wails build -platform windows/amd64

  build-installer:
    desc: Compilar ejecutable e instalador NSIS para Windows
    cmd: wails build -platform windows/amd64 -nsis
```

---

## 8. Hoja de Ruta Recomendada al Iniciar el Proyecto

Cuando se decida comenzar la construcción del repositorio desde cero, la secuencia recomendada es:

1. **Fase 0 — Estructura y Gobernanza:** Inicializar el módulo Go (`go mod init`), crear el árbol hexagonal + Wails, redactar `AGENTS.md`, configurar `.golangci.yml`, `.gomarkdoc.yml`, `Taskfile.yml`, `lefthook.yml` y `README.md`.
2. **Fase 1 — Skills y Documentación de Diseño:** Instalar/crear las skills en `.agents/skills/` y redactar `docs/arquitectura.md`, `docs/seguridad-boveda.md`, `docs/ui-ux-spec.md` y diagramas `.puml`.
3. **Fase 2 — Dominio, Bóveda y Multi-Usuario (TDD):** Implementar entidades en `internal/core/domain`, puertos en `internal/core/ports`, cifrado Argon2id + AES-256-GCM, SQLite con aislamiento por `user_id` y bootstrap del **usuario inicial**.
4. **Fase 3 — Motor SSH + SFTP (TDD):** Implementar adaptadores `sshclient` (conexión automática con llave, `known_hosts` TOFU, PTY) y `sftpclient` (navegación, subida, descarga, progreso).
5. **Fase 4 — Interfaz Desktop Windows (Xshell + Xftp):** Conectar los handlers Wails con el frontend (`xterm.js` en pestañas + panel dual-pane SFTP + gestor de instancias/llaves) y generar el instalador Windows con `task build-installer`.
