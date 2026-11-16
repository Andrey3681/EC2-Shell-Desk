# Plan de Verificación y Pruebas — EC2 Shell Desk

## 1. Niveles de Prueba

| Nivel | Herramienta / Alcance | Criterio de Aceptación |
| :--- | :--- | :--- |
| **Estático & Formato** | `gofumpt -l .`, `golangci-lint run ./...` | 0 advertencias, 0 errores, docstrings completos en inglés. |
| **Unitario (Dominio & Servicios)** | `go test ./internal/core/... ./internal/services/... -race` | Table-driven tests con dobles en memoria; aislamiento estricto por `owner_user_id`. |
| **Criptografía & SQLite** | `go test ./internal/adapters/crypto/... ./internal/adapters/sqlite/... -race` | Argon2id + AES-256-GCM validados; ninguna llave en texto plano en SQLite; migraciones idempotentes. |
| **Integración SSH/SFTP** | Servidor SSH/SFTP embebido en Go (`127.0.0.1:0`) | Autenticación con llaves RSA/Ed25519, TOFU `known_hosts`, PTY resize, upload/download y cancelación sin fuga de goroutines. |
| **Manual Windows + EC2 Real** | App Wails compilada (`task build`) | Validación end-to-end contra instancia AWS EC2 real (`ec2-user` / `ubuntu`). |

## 2. Matriz de Pruebas Funcionales Clave

1. **Multi-usuario:** Usuario A crea llave e instancia -> cambia a Usuario B -> Usuario B no ve ni puede conectar a recursos de Usuario A.
2. **Llaves inválidas / `.ppk`:** Importar archivo corrupto o `.ppk` devuelve mensaje controlado sin crashear ni dejar archivos temporales.
3. **TOFU:** Primera conexión solicita confianza; reconexión entra directo; cambio simulado de clave del servidor bloquea conexión.
4. **Terminal PTY:** Ejecutar `ls --color`, `top`, `vim` y redimensionar ventana verifica `WindowChange` correcto en `xterm.js`.
5. **Transferencias SFTP:** Subir y descargar archivo >10MB desde `C:\` hacia `/home/ec2-user`, cancelar a mitad de transferencia y confirmar que no quedan goroutines colgadas ni UI congelada.
