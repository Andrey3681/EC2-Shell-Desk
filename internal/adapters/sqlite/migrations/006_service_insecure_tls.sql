-- 006_service_insecure_tls.sql: Allow per-service opt-in TLS skip verification for self-signed internal HTTPS endpoints.

ALTER TABLE private_services ADD COLUMN allow_insecure_tls INTEGER NOT NULL DEFAULT 0;
