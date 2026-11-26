# Especificación UI/UX — Estilo Xshell + Xftp

## 1. Pantallas y Flujo de Navegación

1. **Pantalla de Arranque / Login (`AuthView`)**
   - En primer arranque: formulario de creación de **Usuario Inicial** (`Usuario`, `Contraseña`, `Confirmar Contraseña`).
   - En arranques posteriores: selector de usuario local + campo contraseña para desbloquear su bóveda.
2. **Espacio de Trabajo Principal (`WorkspaceView`)**
   - **Barra Superior (`TopBar`):** Usuario activo, selector de idioma (`ES` / `EN`), botón **Bóveda de Llaves**, botón **Bloquear Sesión**.
   - **Panel Lateral Izquierdo (4 Pestañas):**
     - **Instancias (`InstanceSidebar`):** Grupos plegables definidos por el usuario, buscador, filtro por entorno (`Prod`, `Staging`, `Dev`), botón `+ Nueva Instancia` y accesos rápidos (`Conectar`, `Snippets`, `Servicios (N)`, `Editar`, `Eliminar`).
     - **Túneles (`TunnelSidebar`):** Reenvíos SSH locales para bases de datos con estado en vivo, conexiones activas y tráfico.
     - **Snippets (`SnippetSidebar`):** Comandos filtrables por instancia activa y grupo, con ejecución individual y difusión a todas las sesiones.
     - **Servicios (`ServiceSidebar`):** Portal de servicios privados de observabilidad con estado de salud HTTP, latencia y apertura en navegador en 1 clic.
   - **Área Central (`TerminalTabs`):**
     - Pestañas por cada sesión SSH abierta (estilo Xshell) con indicador de estado y botón de cierre limpio.
     - Emulador `xterm.js` con soporte ANSI/VT100, copiar/pegar rápido, ajuste automático de fuente y `FitAddon` al redimensionar la ventana.
     - Botón toggle en cabecera de pestaña sincronizado con el cierre interno del **Panel SFTP (Xftp)**.
   - **Gestor de Archivos SFTP (`XftpDualPane` — Modern Commander):**
     - **Modos de vista configurables (persistentes):** **Abajo** (con divisor horizontal arrastrable), **Flotante** (movible y redimensionable sobre la terminal) y **Maximizado** (manteniendo las pestañas SSH visibles arriba), siempre con badge de la instancia activa.
     - **Panel Local (Windows) ↔ Panel Remoto (EC2/Linux):** Migas de pan clicables con edición de ruta, iconos SVG por tipo de archivo, filtro rápido, selección múltiple, barra superior contextual sin botones repetitivos por fila, *drag & drop* entre paneles y botón dedicado **`Subir carpeta`** recursivo.
     - **Cola de Transferencias (`TransferQueue`):** Sección colapsable inferior con barra de progreso, estado y botón `Cancelar`.
3. **Modales Clave**
   - **Modal Bóveda de Llaves (`KeyVaultModal`):** Importar `.pem` / OpenSSH mediante diálogo nativo Windows, ver huella SHA-256 y eliminar llaves no usadas o desvincularlas.
   - **Modal Verificación TOFU (`HostKeyTrustModal`):** Muestra host, puerto, algoritmo y huella SHA-256 en primera conexión o alerta roja si la huella cambió.

## 2. Principios Visuales

- Tema oscuro profesional orientado a terminales de alto contraste y lectura prolongada.
- Feedback inmediato en acciones destructivas (confirmación al eliminar llaves, instancias o archivos remotos).
- Textos desacoplados en `frontend/src/i18n/es.ts` y `frontend/src/i18n/en.ts`.
