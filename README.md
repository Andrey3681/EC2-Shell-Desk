# EC2 Shell Desk

[![Windows CI](https://github.com/Andrey3681/EC2-Shell-Desk/actions/workflows/ci.yml/badge.svg)](https://github.com/Andrey3681/EC2-Shell-Desk/actions/workflows/ci.yml)

Aplicación de escritorio nativa y liviana para **Windows** escrita en **Go + Wails v2**, inspirada en **Xshell + Xftp**, enfocada en simplificar la conexión **SSH** y la transferencia de archivos **SFTP** hacia **instancias AWS EC2 y servidores Linux**.

> [!NOTE]
> **Estado del Proyecto:** Fases 0 a 4 implementadas y compiladas para Windows (`ec2-shell-desk.exe`). Pendiente validación manual contra instancia AWS EC2 real según [`docs/verificacion.md`](docs/verificacion.md) y [`docs/TODO.md`](docs/TODO.md).

Repositorio oficial: [Andrey3681/EC2-Shell-Desk](https://github.com/Andrey3681/EC2-Shell-Desk)

---

## Funcionalidades Principales

- **Usuario Inicial y Perfiles Multi-Usuario Aislados:** Al iniciar por primera vez se configura un usuario inicial local. Cada usuario cuenta con su propia bóveda cifrada, llaves SSH, `known_hosts`, instancias, grupos, túneles, snippets y servicios privados aislados estrictamente por `user_id`, además de retardo progresivo (*exponential backoff*) ante intentos fallidos de acceso.
- **Bóveda Cifrada de Llaves SSH:** Importa llaves privadas (`.pem`, `id_ed25519`, `id_rsa`) cifradas en reposo con **Argon2id + AES-256-GCM** y vincúlalas a cada instancia.
- **Organización por Grupos y Entornos:** Agrupa instancias EC2 / VMs en secciones plegables definidas por el usuario manteniendo filtros rápidos por entorno (`Prod`, `Staging`, `Dev`).
- **Terminal SSH Integrada (Estilo Xshell):** Conexión en 1 clic con soporte PTY completo (`xterm.js`), colores ANSI/VT100, redimensionamiento dinámico y verificación TOFU (*Trust On First Use*).
- **Gestor SFTP Modern Commander (Estilo Xftp):** Explorador dual-pane Local (Windows) ↔ Remoto (Linux/EC2) con migas de pan clicables, iconos SVG por tipo de archivo, filtro rápido, selección múltiple, *drag & drop*, **subida recursiva de carpetas completas** y 3 modos de vista persistentes (`Abajo` redimensionable, `Flotante` movible y `Maximizado`) siempre sincronizados con la pestaña SSH activa.
- **Túneles SSH para Bases de Datos:** Reenvío de puertos locales sobre SSH con plantillas para PostgreSQL/RDS, MySQL/Aurora, MongoDB/DocumentDB, Redis/ElastiCache, SQL Server y TCP personalizado, multiplexando conexiones sobre clientes SSH compartidos por host.
- **Snippets de Comandos Reutilizables:** Biblioteca de comandos por instancia, por grupo o globales con ejecución inmediata (`Auto-Enter`), pegado seguro y difusión (*broadcast*) hacia múltiples terminales abiertas.
- **Portal de Servicios Privados y Observabilidad:** Acceso en 1 clic en el navegador a paneles internos (Grafana, Prometheus, Loki, Mailpit, Portainer, Netdata, Uptime Kuma o endpoints HTTP/HTTPS propios detrás de Nginx/Traefik) sobre túnel SSH loopback sin abrir puertos en los Security Groups de AWS, con sondeos de salud HTTP/HTTPS y TLS estricto por defecto (con opción explícita por servicio para certificados autofirmados).

## Modelo de Seguridad Resumido

- **100% Local:** Sin telemetría, sin sincronización en la nube ni envío de llaves privadas fuera de tu equipo.
- **Cifrado en Reposo:** La contraseña local deriva una clave maestra mediante **Argon2id**; las llaves `.pem` se cifran con **AES-256-GCM** y AAD vinculado al usuario y llave.
- **Verificación de Host y Endurecimiento:** Política *TOFU* con almacenamiento de huellas por usuario (`known_hosts`), retardo progresivo tras intentos fallidos de login y verificación TLS 1.2+ en sondeos de observabilidad. Consulta [`docs/seguridad-boveda.md`](docs/seguridad-boveda.md).

## Requisitos en Windows

- **Go** `1.27+` (ver `go.mod`)
- **Wails CLI** `v2` (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- **Node.js** `20 LTS+` y `npm`
- **Microsoft Edge WebView2 Runtime** (preinstalado en Windows 10/11)
- **Task** (`go-task`) para automatización
- **NSIS** (opcional, requerido únicamente para `task build-installer`)

## Comandos Rápidos (`Taskfile.yml`)

```powershell
# Clonar el repositorio
git clone https://github.com/Andrey3681/EC2-Shell-Desk.git
cd EC2-Shell-Desk

# Verificar herramientas instaladas en Windows
task doctor

# Descargar dependencias e instalar herramientas de calidad (Wails, gofumpt, golangci-lint, gomarkdoc)
task setup

# Iniciar aplicación de escritorio en modo desarrollo (hot-reload)
task dev

# Validación integral (formato + lint + tests + docs)
task check

# Compilar binario portátil (.exe) para Windows
task build

# Compilar instalador NSIS para Windows
task build-installer
```

## Arquitectura y Documentación

El backend en Go sigue **Arquitectura Hexagonal (Puertos y Adaptadores)** separando el dominio puro (`internal/core/`) de los servicios (`internal/services/`), los adaptadores (`internal/adapters/`: `sqlite`, `crypto`, `sshclient`, `sftpclient`, `winfs`, `httpprobe`) y los bindings Wails (`internal/app/`).

- [`docs/arquitectura.md`](docs/arquitectura.md) — Capas hexagonales, concurrencia e IPC con Wails
- [`docs/diseno-solucion.md`](docs/diseno-solucion.md) — Flujos funcionales y casos de uso
- [`docs/seguridad-boveda.md`](docs/seguridad-boveda.md) — Modelo criptográfico Argon2id + AES-256-GCM y TOFU
- [`docs/ui-ux-spec.md`](docs/ui-ux-spec.md) — Especificación visual estilo Xshell + Xftp
- [`docs/verificacion.md`](docs/verificacion.md) — Estrategia de pruebas unitarias, integración y manuales
- [`docs/TODO.md`](docs/TODO.md) — Hoja de ruta y lista de verificación por fases
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — Flujo Git, Conventional Commits y estándares de código

## Licencia

Este proyecto es **Source-Available (Uso No Comercial)** — **Todos los derechos reservados a [Andrey3681](https://github.com/Andrey3681)**.

- El código fuente es público para consulta y uso **estrictamente personal, educativo o sin ánimo de lucro**.
- Queda prohibido el uso comercial, la venta, monetización o redistribución sin autorización previa y por escrito del autor. Consulta [`LICENSE`](LICENSE) para conocer los términos completos.
