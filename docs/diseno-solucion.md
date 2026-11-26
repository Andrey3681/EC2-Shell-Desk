# Diseño de Solución y Casos de Uso — EC2 Shell Desk

## 1. Alcance Funcional ("Sin Bloat")

La aplicación resuelve el flujo operativo diario sobre instancias EC2 y servidores Linux:
**Usuario Local → Bóveda de Llaves SSH → Grupos e Instancias EC2 → Terminal SSH + SFTP Modern Commander + Túneles DB + Snippets + Portal de Observabilidad Privada**.

Quedan fuera de alcance protocolos heredados como Telnet, Serial/COM, Rlogin, X11 forwarding o motores de scripting propietarios.

## 2. Casos de Uso Principales

### UC-01: Arranque Inicial y Gestión de Usuarios Locales
1. Al iniciar la app, `AuthService` consulta si existen usuarios en SQLite.
2. Si `count == 0`, el frontend muestra el asistente de creación del **usuario inicial** (`username` + `password`).
3. Al autenticar, `AuthService` verifica la contraseña mediante **Argon2id**, deriva la clave maestra de bóveda en memoria y desbloquea la sesión del usuario activo.
4. Es posible crear usuarios adicionales desde el selector de perfil; cada uno inicia con su propia bóveda vacía e independiente.

### UC-02: Importación y Bóveda de Llaves SSH
1. El usuario abre el modal de bóveda y selecciona un archivo `.pem`, `id_ed25519` o `id_rsa` mediante el diálogo nativo de Windows (`runtime.OpenFileDialog`).
2. Opcionalmente ingresa un nombre legible (ej. `aws-prod-east.pem`) y su *passphrase*.
3. Si el archivo corresponde a un formato PuTTY `.ppk`, el sistema lo detecta explícitamente y muestra guía clara para convertirlo o soportarlo según versión.
4. `VaultService` calcula la huella digital pública (*fingerprint* SHA-256), cifra el material privado con **AES-256-GCM** usando la clave maestra del usuario en sesión y persiste el registro en `ssh_keys` vinculado a `owner_user_id`.

### UC-03: Registro y Organización de Instancias EC2 / VMs
1. El usuario crea una instancia especificando:
   - `Alias` (ej. `API Production`)
   - `HostAddress` (IP pública/privada o DNS `ec2-*.amazonaws.com`)
   - `Port` (por defecto `22`)
   - `SSHUsername` (`ec2-user`, `ubuntu`, `admin`, `debian`, `rocky`, etc.)
   - `SSHKeyID` (referencia a una llave de la bóveda del mismo usuario)
   - `Environment` (`Prod`, `Staging`, `Dev`) y `Notes`
2. Todas las operaciones CRUD validan estrictamente que `owner_user_id == activeUser.ID`.

### UC-04: Conexión SSH en 1 Clic (Modo Xshell)
1. El usuario hace doble clic sobre una instancia en la barra lateral.
2. `SessionService` recupera la instancia y descifra en memoria únicamente la llave vinculada.
3. Durante el handshake SSH, el callback *TOFU* consulta `known_hosts` del usuario:
   - Si es el primer acceso al host, solicita confirmación visual mostrando la huella SHA-256 del servidor.
   - Si la huella cambió respecto a la guardada, aborta inmediatamente con alerta de seguridad.
4. Se abre un PTY (`xterm-256color`) y aparece una nueva pestaña interactiva en `xterm.js`.

### UC-05: Explorador Modern Commander SFTP (Modo Xftp)
1. Sobre cualquier sesión conectada, el usuario abre el gestor SFTP en modo **Acoplado inferior redimensionable**, **Flotante movible** o **Maximizado** (manteniendo siempre sincronizado el contexto de la pestaña SSH activa).
2. Panel izquierdo: sistema de archivos local de Windows (`C:\`, `D:\`, migas de pan interactivas, filtro rápido, selección múltiple).
3. Panel derecho: sistema de archivos remoto en la instancia EC2 (migas de pan POSIX, permisos, tamaño, filtro rápido).
4. Acciones soportadas: navegar directorios, **subir carpetas completas recursivamente** (`MkdirAll` + streaming acumulado), subir archivos (diálogo nativo, selección múltiple o *drag & drop*), descargar a Windows, crear carpeta (`Mkdir`), renombrar (`Rename`), eliminar (`Remove`) y cancelar transferencias en curso.

### UC-06: Grupos de Instancias y Snippets de Comandos
1. El usuario crea grupos (`HostGroup`) para organizar instancias por proyecto o cliente.
2. En la pestaña **Snippets**, define comandos reutilizables con alcance por instancia específica, por grupo o globales, eligiendo entre ejecución automática (`Auto-Enter`) o pegado seguro.
3. Puede ejecutar un snippet en la terminal activa o difundirlo (*broadcast*) hacia todas las sesiones SSH abiertas.

### UC-07: Túneles SSH de Base de Datos
1. En la pestaña **Túneles**, el usuario registra reenvíos locales hacia AWS RDS, DocumentDB, ElastiCache o servicios TCP internos usando una instancia EC2 como bastión.
2. Al iniciar varios túneles hacia el mismo bastión, `sshclient` multiplexa los canales sobre una única conexión SSH compartida, reportando conexiones activas y bytes transferidos en vivo.

### UC-08: Portal de Servicios Privados y Observabilidad
1. En la pestaña **Servicios** (o desde el acceso rápido `Servicios (N)` de cada instancia), el usuario registra servicios internos (Grafana, Prometheus, Loki, Mailpit, Portainer, Netdata, Uptime Kuma o HTTP/HTTPS personalizado con subruta Nginx/Traefik).
2. Con 1 clic, `ObservabilityService` levanta un túnel loopback en `127.0.0.1`, ejecuta un sondeo de salud mediante el puerto `HealthProber` (TLS 1.2+ estricto por defecto u opt-in `AllowInsecureTLS` para certificados autofirmados) y abre el servicio en el navegador local sin exponer puertos de la EC2 a Internet.

## 3. Diagramas de Flujo

- [`docs/diagramas/flujo-boveda-llaves.puml`](diagramas/flujo-boveda-llaves.puml)
- [`docs/diagramas/flujo-ssh-sftp.puml`](diagramas/flujo-ssh-sftp.puml)
