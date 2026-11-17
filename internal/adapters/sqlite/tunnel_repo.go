package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ec2-shell-desk/internal/core/domain"
)

// TunnelRepository implements ports.TunnelRepository backed by SQLite with strict owner_user_id isolation.
type TunnelRepository struct {
	db *sql.DB
}

// NewTunnelRepository creates a TunnelRepository using the provided SQLite Store.
func NewTunnelRepository(store *Store) *TunnelRepository {
	return &TunnelRepository{db: store.db}
}

// Create inserts a new SSHTunnel owned by tunnel.OwnerUserID.
func (r *TunnelRepository) Create(ctx context.Context, tunnel domain.SSHTunnel) error {
	const query = `
		INSERT INTO ssh_tunnels (
			id, owner_user_id, host_id, name, preset, local_host, local_port,
			remote_host, remote_port, notes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(
		ctx,
		query,
		tunnel.ID,
		tunnel.OwnerUserID,
		tunnel.HostID,
		tunnel.Name,
		string(tunnel.Preset),
		tunnel.LocalHost,
		tunnel.LocalPort,
		tunnel.RemoteHost,
		tunnel.RemotePort,
		tunnel.Notes,
		formatTime(tunnel.CreatedAt),
		formatTime(tunnel.UpdatedAt),
	)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return fmt.Errorf("%w: tunnel name %q already exists", domain.ErrDuplicateRecord, tunnel.Name)
		}
		return fmt.Errorf("insert ssh tunnel: %w", err)
	}
	return nil
}

// GetByID retrieves an SSHTunnel only if it belongs to ownerUserID.
func (r *TunnelRepository) GetByID(ctx context.Context, ownerUserID, tunnelID string) (domain.SSHTunnel, error) {
	const query = `
		SELECT id, owner_user_id, host_id, name, preset, local_host, local_port,
		       remote_host, remote_port, notes, created_at, updated_at
		FROM ssh_tunnels
		WHERE id = ? AND owner_user_id = ?`

	tunnel, err := scanTunnelRow(r.db.QueryRowContext(ctx, query, tunnelID, ownerUserID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.SSHTunnel{}, fmt.Errorf("%w: ssh tunnel", domain.ErrNotFound)
		}
		return domain.SSHTunnel{}, err
	}
	return tunnel, nil
}

// ListByUser returns all SSHTunnel records belonging strictly to ownerUserID.
func (r *TunnelRepository) ListByUser(ctx context.Context, ownerUserID string) ([]domain.SSHTunnel, error) {
	const query = `
		SELECT id, owner_user_id, host_id, name, preset, local_host, local_port,
		       remote_host, remote_port, notes, created_at, updated_at
		FROM ssh_tunnels
		WHERE owner_user_id = ?
		ORDER BY name COLLATE NOCASE ASC`

	rows, err := r.db.QueryContext(ctx, query, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list ssh tunnels: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	tunnels := make([]domain.SSHTunnel, 0)
	for rows.Next() {
		tun, scanErr := scanTunnelRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		tunnels = append(tunnels, tun)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ssh tunnels: %w", err)
	}
	return tunnels, nil
}

// Update modifies an existing SSHTunnel only if owned by tunnel.OwnerUserID.
func (r *TunnelRepository) Update(ctx context.Context, tunnel domain.SSHTunnel) error {
	const query = `
		UPDATE ssh_tunnels
		SET host_id = ?, name = ?, preset = ?, local_host = ?, local_port = ?,
		    remote_host = ?, remote_port = ?, notes = ?, updated_at = ?
		WHERE id = ? AND owner_user_id = ?`

	res, err := r.db.ExecContext(
		ctx,
		query,
		tunnel.HostID,
		tunnel.Name,
		string(tunnel.Preset),
		tunnel.LocalHost,
		tunnel.LocalPort,
		tunnel.RemoteHost,
		tunnel.RemotePort,
		tunnel.Notes,
		formatTime(tunnel.UpdatedAt),
		tunnel.ID,
		tunnel.OwnerUserID,
	)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return fmt.Errorf("%w: tunnel name %q already exists", domain.ErrDuplicateRecord, tunnel.Name)
		}
		return fmt.Errorf("update ssh tunnel: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated tunnel rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: ssh tunnel", domain.ErrNotFound)
	}
	return nil
}

// Delete removes an SSHTunnel only if owned by ownerUserID.
func (r *TunnelRepository) Delete(ctx context.Context, ownerUserID, tunnelID string) error {
	const query = `DELETE FROM ssh_tunnels WHERE id = ? AND owner_user_id = ?`
	res, err := r.db.ExecContext(ctx, query, tunnelID, ownerUserID)
	if err != nil {
		return fmt.Errorf("delete ssh tunnel: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted tunnel rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: ssh tunnel", domain.ErrNotFound)
	}
	return nil
}

// CountByHost returns how many saved tunnels owned by ownerUserID reference hostID.
func (r *TunnelRepository) CountByHost(ctx context.Context, ownerUserID, hostID string) (int, error) {
	const query = `SELECT COUNT(1) FROM ssh_tunnels WHERE owner_user_id = ? AND host_id = ?`
	var total int
	if err := r.db.QueryRowContext(ctx, query, ownerUserID, hostID).Scan(&total); err != nil {
		return 0, fmt.Errorf("count tunnels by host: %w", err)
	}
	return total, nil
}

func scanTunnelRow(scanner rowScanner) (domain.SSHTunnel, error) {
	var (
		t         domain.SSHTunnel
		preset    string
		createdAt string
		updatedAt string
	)
	if err := scanner.Scan(
		&t.ID,
		&t.OwnerUserID,
		&t.HostID,
		&t.Name,
		&preset,
		&t.LocalHost,
		&t.LocalPort,
		&t.RemoteHost,
		&t.RemotePort,
		&t.Notes,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.SSHTunnel{}, fmt.Errorf("scan ssh tunnel row: %w", err)
	}
	t.Preset = domain.TunnelPreset(preset)

	var err error
	if t.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.SSHTunnel{}, err
	}
	if t.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.SSHTunnel{}, err
	}
	return t, nil
}
