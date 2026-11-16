# Hoja de Ruta y Lista de Verificación — EC2 Shell Desk

## Fase 0 — Estructura y Gobernanza
- [x] Inicializar repositorio Git y módulo Go (`go mod init ec2-shell-desk`)
- [x] Crear árbol hexagonal (`cmd/desktop`, `internal/core/{domain,ports}`, `internal/services`, `internal/adapters/*`, `internal/app`) y esqueleto `frontend/`
- [x] Redactar `AGENTS.md` y reglas modulares en `.agents/rules/`
- [x] Configurar `.golangci.yml`, `.gomarkdoc.yml`, `Taskfile.yml`, `lefthook.yml`, `.gitignore` y `wails.json`
- [x] Crear scripts Windows (`scripts/dev-doctor.ps1`, `scripts/verify-commit-msg.ps1`) y CI (`.github/workflows/ci.yml`)
- [x] Crear `README.md`, `CONTRIBUTING.md` y `CHANGELOG.md`

## Fase 1 — Skills y Documentación de Diseño
- [x] Crear catálogo de skills en `.agents/skills/` (11 skills de dominio, Go, Wails, seguridad, QA y docs)
- [x] Redactar `docs/arquitectura.md`
- [x] Redactar `docs/diseno-solucion.md`
- [x] Redactar `docs/seguridad-boveda.md`
- [x] Redactar `docs/ui-ux-spec.md`
- [x] Redactar `docs/verificacion.md`
- [x] Crear diagramas PlantUML en `docs/diagramas/*.puml`

## Fase 2 — Dominio, Bóveda y Multi-Usuario (TDD)
- [x] Definir entidades y errores centinela en `internal/core/domain/`
- [x] Definir puertos en `internal/core/ports/` (`repository.go`, `crypto.go`, `ssh_engine.go`, `sftp_engine.go`, `local_fs.go`)
- [x] Implementar adaptador `internal/adapters/crypto` (Argon2id + AES-256-GCM + limpieza en memoria)
- [x] Implementar adaptador `internal/adapters/sqlite` con migraciones embebidas y filtro estricto `owner_user_id`
- [x] Implementar `auth_service.go`, `vault_service.go` y `host_service.go` (incluyendo bootstrap del usuario inicial)
- [x] Verificar aislamiento multi-usuario y cobertura de pruebas unitarias (`task test`)

## Fase 3 — Motor SSH + SFTP (TDD)
- [x] Implementar `internal/adapters/sshclient` (RSA/Ed25519/ECDSA PEM, detección `.ppk`, TOFU `known_hosts`, PTY + keepalive)
- [x] Implementar `internal/adapters/sftpclient` (listado, mkdir, rename, remove, streaming upload/download con cancelación)
- [x] Implementar `internal/adapters/winfs` (listado de unidades `C:\`, `D:\` y navegación segura en Windows)
- [x] Implementar `session_service.go` y `transfer_service.go` con cola de transferencias y eventos
- [x] Pruebas de integración contra servidor SSH/SFTP embebido en Go

## Fase 4 — Interfaz Desktop Windows (Xshell + Xftp) y Empaquetado
- [x] Conectar bindings Wails en `internal/app/*_handler.go` y DI en `cmd/desktop/main.go`
- [x] Implementar UI en `frontend/` (Login/Bootstrap, Sidebar Instancias + Bóveda `.pem`, Terminal Tabs `xterm.js`, SFTP Dual-Pane)
- [x] Soporte bilingüe en `frontend/src/i18n/` (`es` / `en`)
- [ ] Validar prueba manual en Windows contra una instancia EC2 real (`docs/verificacion.md`)
- [x] Compilar `.exe` portátil (`task build`) (instalador NSIS opcional con `task build-installer` si NSIS está instalado)
