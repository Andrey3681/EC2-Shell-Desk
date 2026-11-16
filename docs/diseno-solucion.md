# Diseño de Solución y Casos de Uso — EC2 Shell Desk

## 1. Alcance Funcional ("Sin Bloat")

La aplicación resuelve exclusivamente el flujo:
**Usuario Local → Bóveda de Llaves SSH → Instancias EC2 / VMs Linux → Terminal SSH + SFTP Dual-Pane**.

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

### UC-05: Explorador Dual-Pane SFTP (Modo Xftp)
1. Sobre cualquier sesión conectada, el usuario abre o expande el panel SFTP acoplado.
2. Panel izquierdo: sistema de archivos local de Windows (`C:\`, `D:\`, carpetas de usuario).
3. Panel derecho: sistema de archivos remoto en la instancia EC2.
4. Acciones soportadas: navegar directorios, subir documentos (botón o *drag & drop*), descargar a Windows, crear carpeta (`Mkdir`), renombrar (`Rename`), eliminar (`Remove`) y cancelar transferencias en curso.

## 3. Diagramas de Flujo

- [`docs/diagramas/flujo-boveda-llaves.puml`](diagramas/flujo-boveda-llaves.puml)
- [`docs/diagramas/flujo-ssh-sftp.puml`](diagramas/flujo-ssh-sftp.puml)
