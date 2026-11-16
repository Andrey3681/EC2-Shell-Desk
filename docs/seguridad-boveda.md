# Modelo de Seguridad y Bóveda — EC2 Shell Desk

## 1. Principios de Seguridad

1. **Cero secretos en texto plano:** Ninguna llave privada (`.pem`, `id_ed25519`, `id_rsa`), passphrase o contraseña de usuario se almacena en texto plano en SQLite ni en archivos temporales del disco.
2. **Aislamiento estricto por usuario (`owner_user_id`):** Cada usuario posee su propio *salt* criptográfico, clave maestra derivada, llaves SSH, instancias y tabla de confianza `known_hosts`.
3. **Cero telemetría y cero logs sensibles:** Los logs estructurados (`log/slog`) jamás registran llaves, passphrases, contraseñas ni contenido de archivos transferidos.

## 2. Derivación de Claves y Cifrado de Bóveda

```text
Contraseña del Usuario + vault_salt (16 bytes aleatorios)
          │
          ▼  Argon2id (t=3, m=64 MiB, p=4, keyLen=32 bytes)
   Clave Maestra de Sesión (256 bits, solo en RAM)
          │
          ▼  AES-256-GCM (nonce aleatorio 12 bytes + AAD="userID:keyID")
   encrypted_pem / encrypted_passphrase en SQLite
```

### Detalles Técnicos
- **Verificación de Login (`password_hash` + `auth_salt`):** Se usa un `auth_salt` independiente del `vault_salt` para verificar la contraseña del usuario mediante comparación en tiempo constante (`subtle.ConstantTimeCompare`).
- **Authenticated Encryption (AES-256-GCM):** Garantiza confidencialidad e integridad. El parámetro AAD (`ownerUserID:keyID`) impide mover un registro cifrado de un usuario a otro.
- **Bloqueo y Limpieza de Memoria:** Al cerrar sesión, bloquear la bóveda o cerrar la aplicación, los buffers de clave maestra y llaves descifradas se sobrescriben con ceros (`ZeroBytes`) antes de liberar referencias.

## 3. Verificación de Host Remoto (`known_hosts` TOFU)

- Se prohíbe `ssh.InsecureIgnoreHostKey()` en rutas de producción.
- Al conectar a `(host_address, port)`:
  1. **Host desconocido:** Se calcula `ssh.FingerprintSHA256(key)` y se solicita confirmación explícita al usuario antes de guardar la clave pública en `known_hosts`.
  2. **Host conocido coincidente:** La conexión continúa automáticamente.
  3. **Host conocido con clave distinta:** Se bloquea la conexión con error crítico (`ErrHostKeyMismatch`) para prevenir ataques Man-in-the-Middle (MITM) o alertar cambios de instancia reutilizada por IP elástica.

## 4. Almacenamiento en Windows

- La base SQLite (`vault.db`) se ubica en `%APPDATA%\EC2ShellDesk\vault.db` con modo `WAL` y `PRAGMA foreign_keys = ON`.
- Como endurecimiento adicional futuro, puede integrarse **Windows DPAPI** (`CryptProtectData`) para protección opcional de sesión en el equipo local.
