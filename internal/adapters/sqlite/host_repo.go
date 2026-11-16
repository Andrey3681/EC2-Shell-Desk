package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"ec2-shell-desk/internal/core/domain"
)

// HostRepository implements ports.HostRepository backed by SQLite with strict owner_user_id isolation.
type HostRepository struct {
	db *sql.DB
}

// NewHostRepository creates a HostRepository using the provided SQLite Store.
func NewHostRepository(store *Store) *HostRepository {
	return &HostRepository{db: store.db}
}

// Create inserts a new HostInstance owned by host.OwnerUserID.
func (r *HostRepository) Create(ctx context.Context, host domain.HostInstance) error {
	const query = `
		INSERT INTO host_instances (
			id, owner_user_id, alias, host_address, port, ssh_username, ssh_key_id,
			environment, notes, last_connected_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var keyID sql.NullString
	if host.SSHKeyID != "" {
		keyID = sql.NullString{String: host.SSHKeyID, Valid: true}
	}

	var lastConnected sql.NullString
	if host.LastConnectedAt != nil {
		lastConnected = sql.NullString{String: formatTime(*host.LastConnectedAt), Valid: true}
	}

	_, err := r.db.ExecContext(
		ctx,
		query,
		host.ID,
		host.OwnerUserID,
		host.Alias,
		host.HostAddress,
		host.Port,
		host.SSHUsername,
		keyID,
		string(host.Environment),
		host.Notes,
		lastConnected,
		formatTime(host.CreatedAt),
		formatTime(host.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert host instance: %w", err)
	}
	return nil
}

// GetByID retrieves a HostInstance only if it belongs to ownerUserID.
func (r *HostRepository) GetByID(ctx context.Context, ownerUserID, hostID string) (domain.HostInstance, error) {
	const query = `
		SELECT id, owner_user_id, alias, host_address, port, ssh_username, ssh_key_id,
		       environment, notes, last_connected_at, created_at, updated_at
		FROM host_instances
		WHERE id = ? AND owner_user_id = ?`

	host, err := scanHostRow(r.db.QueryRowContext(ctx, query, hostID, ownerUserID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.HostInstance{}, fmt.Errorf("%w: host instance", domain.ErrNotFound)
		}
		return domain.HostInstance{}, err
	}
	return host, nil
}

// ListByUser returns all HostInstance records belonging strictly to ownerUserID.
func (r *HostRepository) ListByUser(ctx context.Context, ownerUserID string) ([]domain.HostInstance, error) {
	const query = `
		SELECT id, owner_user_id, alias, host_address, port, ssh_username, ssh_key_id,
		       environment, notes, last_connected_at, created_at, updated_at
		FROM host_instances
		WHERE owner_user_id = ?
		ORDER BY alias COLLATE NOCASE ASC`

	rows, err := r.db.QueryContext(ctx, query, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list host instances: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	hosts := make([]domain.HostInstance, 0)
	for rows.Next() {
		h, err := scanHostRow(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate host instances: %w", err)
	}
	return hosts, nil
}

// Update modifies an existing HostInstance only if owned by host.OwnerUserID.
func (r *HostRepository) Update(ctx context.Context, host domain.HostInstance) error {
	const query = `
		UPDATE host_instances
		SET alias = ?, host_address = ?, port = ?, ssh_username = ?, ssh_key_id = ?,
		    environment = ?, notes = ?, updated_at = ?
		WHERE id = ? AND owner_user_id = ?`

	var keyID sql.NullString
	if host.SSHKeyID != "" {
		keyID = sql.NullString{String: host.SSHKeyID, Valid: true}
	}

	res, err := r.db.ExecContext(
		ctx,
		query,
		host.Alias,
		host.HostAddress,
		host.Port,
		host.SSHUsername,
		keyID,
		string(host.Environment),
		host.Notes,
		formatTime(host.UpdatedAt),
		host.ID,
		host.OwnerUserID,
	)
	if err != nil {
		return fmt.Errorf("update host instance: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated host rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: host instance", domain.ErrNotFound)
	}
	return nil
}

// Delete removes a HostInstance only if owned by ownerUserID.
func (r *HostRepository) Delete(ctx context.Context, ownerUserID, hostID string) error {
	const query = `DELETE FROM host_instances WHERE id = ? AND owner_user_id = ?`
	res, err := r.db.ExecContext(ctx, query, hostID, ownerUserID)
	if err != nil {
		return fmt.Errorf("delete host instance: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted host rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: host instance", domain.ErrNotFound)
	}
	return nil
}

// TouchLastConnected updates the last_connected_at timestamp for a user-owned host.
func (r *HostRepository) TouchLastConnected(ctx context.Context, ownerUserID, hostID string, connectedAt time.Time) error {
	const query = `UPDATE host_instances SET last_connected_at = ? WHERE id = ? AND owner_user_id = ?`
	res, err := r.db.ExecContext(ctx, query, formatTime(connectedAt), hostID, ownerUserID)
	if err != nil {
		return fmt.Errorf("touch host last_connected_at: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check touched host rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: host instance", domain.ErrNotFound)
	}
	return nil
}

func scanHostRow(scanner rowScanner) (domain.HostInstance, error) {
	var (
		h             domain.HostInstance
		sshKeyID      sql.NullString
		environment   string
		lastConnected sql.NullString
		createdAt     string
		updatedAt     string
	)
	if err := scanner.Scan(
		&h.ID,
		&h.OwnerUserID,
		&h.Alias,
		&h.HostAddress,
		&h.Port,
		&h.SSHUsername,
		&sshKeyID,
		&environment,
		&h.Notes,
		&lastConnected,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.HostInstance{}, fmt.Errorf("scan host instance row: %w", err)
	}
	if sshKeyID.Valid {
		h.SSHKeyID = sshKeyID.String
	}
	h.Environment = domain.Environment(environment)

	var err error
	if lastConnected.Valid {
		parsed, parseErr := parseTime(lastConnected.String)
		if parseErr != nil {
			return domain.HostInstance{}, parseErr
		}
		h.LastConnectedAt = &parsed
	}
	if h.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.HostInstance{}, err
	}
	if h.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.HostInstance{}, err
	}
	return h, nil
}
