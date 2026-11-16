# Especificación UI/UX — Estilo Xshell + Xftp

## 1. Pantallas y Flujo de Navegación

1. **Pantalla de Arranque / Login (`AuthView`)**
   - En primer arranque: formulario de creación de **Usuario Inicial** (`Usuario`, `Contraseña`, `Confirmar Contraseña`).
   - En arranques posteriores: selector de usuario local + campo contraseña para desbloquear su bóveda.
2. **Espacio de Trabajo Principal (`WorkspaceView`)**
   - **Barra Superior (`TopBar`):** Usuario activo, selector de idioma (`ES` / `EN`), botón **Bóveda de Llaves**, botón **Bloquear Sesión**.
   - **Panel Lateral Izquierdo (`InstanceSidebar`):**
     - Buscador rápido de instancias y filtro por entorno (`Prod`, `Staging`, `Dev`).
     - Lista de instancias EC2 con estado de conexión, usuario SSH y llave asociada.
     - Botón `+ Nueva Instancia` y acciones rápidas (`Conectar SSH`, `Editar`, `Eliminar`).
   - **Área Central (`TerminalTabs`):**
     - Pestañas por cada sesión SSH abierta (estilo Xshell) con indicador de estado y botón de cierre limpio.
     - Emulador `xterm.js` con soporte ANSI/VT100, copiar/pegar rápido, ajuste automático de fuente y `FitAddon` al redimensionar la ventana.
     - Botón toggle en cabecera de pestaña para mostrar/ocultar el **Panel SFTP (Xftp)**.
   - **Panel Acoplado / Split Inferior o Lateral (`XftpDualPane`):**
     - **Panel Local (Windows):** Selector de unidad (`C:\`, `D:\`), breadcrumb de ruta, tabla de archivos/carpetas, botón subir y zona *drag & drop*.
     - **Panel Remoto (EC2/Linux):** Breadcrumb POSIX (`/home/ec2-user`), tabla con nombre, tamaño, permisos y fecha, acciones (`Descargar`, `Nueva Carpeta`, `Renombrar`, `Eliminar`).
     - **Cola de Transferencias (`TransferQueue`):** Lista inferior compacta con barra de progreso, velocidad/bytes transferidos, estado y botón `Cancelar`.
3. **Modales Clave**
   - **Modal Bóveda de Llaves (`KeyVaultModal`):** Importar `.pem` / OpenSSH mediante diálogo nativo Windows, ver huella SHA-256 y eliminar llaves no usadas o desvincularlas.
   - **Modal Verificación TOFU (`HostKeyTrustModal`):** Muestra host, puerto, algoritmo y huella SHA-256 en primera conexión o alerta roja si la huella cambió.

## 2. Principios Visuales

- Tema oscuro profesional orientado a terminales de alto contraste y lectura prolongada.
- Feedback inmediato en acciones destructivas (confirmación al eliminar llaves, instancias o archivos remotos).
- Textos desacoplados en `frontend/src/i18n/es.ts` y `frontend/src/i18n/en.ts`.
