package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ec2-shell-desk/internal/core/domain"
)

// KnownHostRepository implements ports.KnownHostRepository backed by SQLite with owner_user_id isolation.
type KnownHostRepository struct {
	db *sql.DB
}

// NewKnownHostRepository creates a KnownHostRepository using the provided SQLite Store.
func NewKnownHostRepository(store *Store) *KnownHostRepository {
	return &KnownHostRepository{db: store.db}
}

// GetByHostPort retrieves a trusted host entry for a specific user, host address, and port.
func (r *KnownHostRepository) GetByHostPort(ctx context.Context, ownerUserID, hostAddress string, port int) (domain.KnownHost, error) {
	const query = `
		SELECT id, owner_user_id, host_address, port, key_type, fingerprint, public_key_blob, trusted_at
		FROM known_hosts
		WHERE owner_user_id = ? AND host_address = ? AND port = ?`

	kh, err := scanKnownHostRow(r.db.QueryRowContext(ctx, query, ownerUserID, hostAddress, port))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.KnownHost{}, fmt.Errorf("%w: known host", domain.ErrNotFound)
		}
		return domain.KnownHost{}, err
	}
	return kh, nil
}

// Upsert stores or updates a trusted host public key fingerprint for a specific user.
func (r *KnownHostRepository) Upsert(ctx context.Context, knownHost domain.KnownHost) error {
	const query = `
		INSERT INTO known_hosts (id, owner_user_id, host_address, port, key_type, fingerprint, public_key_blob, trusted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(owner_user_id, host_address, port) DO UPDATE SET
			key_type = excluded.key_type,
			fingerprint = excluded.fingerprint,
			public_key_blob = excluded.public_key_blob,
			trusted_at = excluded.trusted_at`

	_, err := r.db.ExecContext(
		ctx,
		query,
		knownHost.ID,
		knownHost.OwnerUserID,
		knownHost.HostAddress,
		knownHost.Port,
		knownHost.KeyType,
		knownHost.Fingerprint,
		knownHost.PublicKeyBlob,
		formatTime(knownHost.TrustedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert known host: %w", err)
	}
	return nil
}

// ListByUser returns all trusted host records belonging strictly to ownerUserID.
func (r *KnownHostRepository) ListByUser(ctx context.Context, ownerUserID string) ([]domain.KnownHost, error) {
	const query = `
		SELECT id, owner_user_id, host_address, port, key_type, fingerprint, public_key_blob, trusted_at
		FROM known_hosts
		WHERE owner_user_id = ?
		ORDER BY host_address ASC, port ASC`

	rows, err := r.db.QueryContext(ctx, query, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list known hosts: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	hosts := make([]domain.KnownHost, 0)
	for rows.Next() {
		kh, err := scanKnownHostRow(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, kh)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate known hosts: %w", err)
	}
	return hosts, nil
}

// Delete removes a trusted host entry only if it belongs to ownerUserID.
func (r *KnownHostRepository) Delete(ctx context.Context, ownerUserID, knownHostID string) error {
	const query = `DELETE FROM known_hosts WHERE id = ? AND owner_user_id = ?`
	res, err := r.db.ExecContext(ctx, query, knownHostID, ownerUserID)
	if err != nil {
		return fmt.Errorf("delete known host: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted known host rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: known host", domain.ErrNotFound)
	}
	return nil
}

func scanKnownHostRow(scanner rowScanner) (domain.KnownHost, error) {
	var (
		kh        domain.KnownHost
		trustedAt string
	)
	if err := scanner.Scan(
		&kh.ID,
		&kh.OwnerUserID,
		&kh.HostAddress,
		&kh.Port,
		&kh.KeyType,
		&kh.Fingerprint,
		&kh.PublicKeyBlob,
		&trustedAt,
	); err != nil {
		return domain.KnownHost{}, fmt.Errorf("scan known host row: %w", err)
	}
	var err error
	if kh.TrustedAt, err = parseTime(trustedAt); err != nil {
		return domain.KnownHost{}, err
	}
	return kh, nil
}
