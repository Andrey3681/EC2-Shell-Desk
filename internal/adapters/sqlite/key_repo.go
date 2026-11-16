package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ec2-shell-desk/internal/core/domain"
)

// KeyRepository implements ports.KeyRepository backed by SQLite with strict owner_user_id isolation.
type KeyRepository struct {
	db *sql.DB
}

// NewKeyRepository creates a KeyRepository using the provided SQLite Store.
func NewKeyRepository(store *Store) *KeyRepository {
	return &KeyRepository{db: store.db}
}

// Create stores an encrypted SSH key belonging to key.OwnerUserID.
func (r *KeyRepository) Create(ctx context.Context, key domain.SSHKey) error {
	const query = `
		INSERT INTO ssh_keys (id, owner_user_id, name, key_type, fingerprint, encrypted_pem, encrypted_passphrase, has_passphrase, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	hasPassphrase := 0
	if key.HasPassphrase {
		hasPassphrase = 1
	}

	_, err := r.db.ExecContext(
		ctx,
		query,
		key.ID,
		key.OwnerUserID,
		key.Name,
		string(key.KeyType),
		key.Fingerprint,
		key.EncryptedPEM,
		key.EncryptedPassphrase,
		hasPassphrase,
		formatTime(key.CreatedAt),
	)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return fmt.Errorf("%w: ssh key name %q already exists", domain.ErrDuplicateRecord, key.Name)
		}
		return fmt.Errorf("insert ssh key: %w", err)
	}
	return nil
}

// GetByID retrieves an encrypted SSH key only if it belongs to ownerUserID.
func (r *KeyRepository) GetByID(ctx context.Context, ownerUserID, keyID string) (domain.SSHKey, error) {
	const query = `
		SELECT id, owner_user_id, name, key_type, fingerprint, encrypted_pem, encrypted_passphrase, has_passphrase, created_at
		FROM ssh_keys
		WHERE id = ? AND owner_user_id = ?`

	key, err := scanKeyRow(r.db.QueryRowContext(ctx, query, keyID, ownerUserID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.SSHKey{}, fmt.Errorf("%w: ssh key", domain.ErrNotFound)
		}
		return domain.SSHKey{}, err
	}
	return key, nil
}

// ListByUser returns all encrypted SSH keys belonging strictly to ownerUserID.
func (r *KeyRepository) ListByUser(ctx context.Context, ownerUserID string) ([]domain.SSHKey, error) {
	const query = `
		SELECT id, owner_user_id, name, key_type, fingerprint, encrypted_pem, encrypted_passphrase, has_passphrase, created_at
		FROM ssh_keys
		WHERE owner_user_id = ?
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list ssh keys: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	keys := make([]domain.SSHKey, 0)
	for rows.Next() {
		k, err := scanKeyRow(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ssh keys: %w", err)
	}
	return keys, nil
}

// Delete removes an SSH key only if it belongs to ownerUserID.
func (r *KeyRepository) Delete(ctx context.Context, ownerUserID, keyID string) error {
	const query = `DELETE FROM ssh_keys WHERE id = ? AND owner_user_id = ?`
	res, err := r.db.ExecContext(ctx, query, keyID, ownerUserID)
	if err != nil {
		return fmt.Errorf("delete ssh key: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted ssh key rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: ssh key", domain.ErrNotFound)
	}
	return nil
}

// CountHostReferences returns how many host instances owned by ownerUserID reference keyID.
func (r *KeyRepository) CountHostReferences(ctx context.Context, ownerUserID, keyID string) (int, error) {
	const query = `SELECT COUNT(1) FROM host_instances WHERE owner_user_id = ? AND ssh_key_id = ?`
	var total int
	if err := r.db.QueryRowContext(ctx, query, ownerUserID, keyID).Scan(&total); err != nil {
		return 0, fmt.Errorf("count key host references: %w", err)
	}
	return total, nil
}

func scanKeyRow(scanner rowScanner) (domain.SSHKey, error) {
	var (
		k             domain.SSHKey
		keyType       string
		hasPassphrase int
		createdAt     string
	)
	if err := scanner.Scan(
		&k.ID,
		&k.OwnerUserID,
		&k.Name,
		&keyType,
		&k.Fingerprint,
		&k.EncryptedPEM,
		&k.EncryptedPassphrase,
		&hasPassphrase,
		&createdAt,
	); err != nil {
		return domain.SSHKey{}, fmt.Errorf("scan ssh key row: %w", err)
	}
	k.KeyType = domain.KeyType(keyType)
	k.HasPassphrase = hasPassphrase == 1

	var err error
	if k.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.SSHKey{}, err
	}
	return k, nil
}
