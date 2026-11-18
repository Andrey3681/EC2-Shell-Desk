package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ec2-shell-desk/internal/core/domain"
)

// ServiceRepository implements ports.ServiceRepository backed by SQLite with strict owner_user_id isolation.
type ServiceRepository struct {
	db *sql.DB
}

// NewServiceRepository creates a ServiceRepository using the provided SQLite Store.
func NewServiceRepository(store *Store) *ServiceRepository {
	return &ServiceRepository{db: store.db}
}

// Create inserts a new PrivateService owned by service.OwnerUserID.
func (r *ServiceRepository) Create(ctx context.Context, service domain.PrivateService) error {
	const query = `
		INSERT INTO private_services (
			id, owner_user_id, host_id, name, preset, protocol, remote_host, remote_port,
			base_path, local_port, health_path, open_mode, notes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(
		ctx,
		query,
		service.ID,
		service.OwnerUserID,
		service.HostID,
		service.Name,
		string(service.Preset),
		string(service.Protocol),
		service.RemoteHost,
		service.RemotePort,
		service.BasePath,
		service.LocalPort,
		service.HealthPath,
		string(service.OpenMode),
		service.Notes,
		formatTime(service.CreatedAt),
		formatTime(service.UpdatedAt),
	)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return fmt.Errorf("%w: service name %q already exists for this host", domain.ErrDuplicateRecord, service.Name)
		}
		return fmt.Errorf("insert private service: %w", err)
	}
	return nil
}

// GetByID retrieves a PrivateService only if it belongs to ownerUserID.
func (r *ServiceRepository) GetByID(ctx context.Context, ownerUserID, serviceID string) (domain.PrivateService, error) {
	const query = `
		SELECT id, owner_user_id, host_id, name, preset, protocol, remote_host, remote_port,
		       base_path, local_port, health_path, open_mode, notes, created_at, updated_at
		FROM private_services
		WHERE id = ? AND owner_user_id = ?`

	svc, err := scanServiceRow(r.db.QueryRowContext(ctx, query, serviceID, ownerUserID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.PrivateService{}, fmt.Errorf("%w: private service", domain.ErrNotFound)
		}
		return domain.PrivateService{}, err
	}
	return svc, nil
}

// ListByUser returns all PrivateService records belonging strictly to ownerUserID.
func (r *ServiceRepository) ListByUser(ctx context.Context, ownerUserID string) ([]domain.PrivateService, error) {
	const query = `
		SELECT id, owner_user_id, host_id, name, preset, protocol, remote_host, remote_port,
		       base_path, local_port, health_path, open_mode, notes, created_at, updated_at
		FROM private_services
		WHERE owner_user_id = ?
		ORDER BY name COLLATE NOCASE ASC`

	rows, err := r.db.QueryContext(ctx, query, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list private services: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	services := make([]domain.PrivateService, 0)
	for rows.Next() {
		s, scanErr := scanServiceRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		services = append(services, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate private services: %w", err)
	}
	return services, nil
}

// ListByHost returns PrivateService records belonging to a specific host and owner.
func (r *ServiceRepository) ListByHost(ctx context.Context, ownerUserID, hostID string) ([]domain.PrivateService, error) {
	const query = `
		SELECT id, owner_user_id, host_id, name, preset, protocol, remote_host, remote_port,
		       base_path, local_port, health_path, open_mode, notes, created_at, updated_at
		FROM private_services
		WHERE owner_user_id = ? AND host_id = ?
		ORDER BY name COLLATE NOCASE ASC`

	rows, err := r.db.QueryContext(ctx, query, ownerUserID, hostID)
	if err != nil {
		return nil, fmt.Errorf("list private services by host: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	services := make([]domain.PrivateService, 0)
	for rows.Next() {
		s, scanErr := scanServiceRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		services = append(services, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate host private services: %w", err)
	}
	return services, nil
}

// Update modifies an existing PrivateService only if owned by service.OwnerUserID.
func (r *ServiceRepository) Update(ctx context.Context, service domain.PrivateService) error {
	const query = `
		UPDATE private_services
		SET host_id = ?, name = ?, preset = ?, protocol = ?, remote_host = ?, remote_port = ?,
		    base_path = ?, local_port = ?, health_path = ?, open_mode = ?, notes = ?, updated_at = ?
		WHERE id = ? AND owner_user_id = ?`

	res, err := r.db.ExecContext(
		ctx,
		query,
		service.HostID,
		service.Name,
		string(service.Preset),
		string(service.Protocol),
		service.RemoteHost,
		service.RemotePort,
		service.BasePath,
		service.LocalPort,
		service.HealthPath,
		string(service.OpenMode),
		service.Notes,
		formatTime(service.UpdatedAt),
		service.ID,
		service.OwnerUserID,
	)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return fmt.Errorf("%w: service name %q already exists for this host", domain.ErrDuplicateRecord, service.Name)
		}
		return fmt.Errorf("update private service: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated service rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: private service", domain.ErrNotFound)
	}
	return nil
}

// Delete removes a PrivateService only if owned by ownerUserID.
func (r *ServiceRepository) Delete(ctx context.Context, ownerUserID, serviceID string) error {
	const query = `DELETE FROM private_services WHERE id = ? AND owner_user_id = ?`
	res, err := r.db.ExecContext(ctx, query, serviceID, ownerUserID)
	if err != nil {
		return fmt.Errorf("delete private service: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted service rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: private service", domain.ErrNotFound)
	}
	return nil
}

// CountByHost returns how many services are associated with a host instance.
func (r *ServiceRepository) CountByHost(ctx context.Context, ownerUserID, hostID string) (int, error) {
	const query = `SELECT COUNT(1) FROM private_services WHERE owner_user_id = ? AND host_id = ?`
	var count int
	if err := r.db.QueryRowContext(ctx, query, ownerUserID, hostID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count private services by host: %w", err)
	}
	return count, nil
}

func scanServiceRow(scanner rowScanner) (domain.PrivateService, error) {
	var (
		s         domain.PrivateService
		preset    string
		protocol  string
		openMode  string
		createdAt string
		updatedAt string
	)
	if err := scanner.Scan(
		&s.ID,
		&s.OwnerUserID,
		&s.HostID,
		&s.Name,
		&preset,
		&protocol,
		&s.RemoteHost,
		&s.RemotePort,
		&s.BasePath,
		&s.LocalPort,
		&s.HealthPath,
		&openMode,
		&s.Notes,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.PrivateService{}, fmt.Errorf("scan private service row: %w", err)
	}
	s.Preset = domain.ServicePreset(preset)
	s.Protocol = domain.ServiceProtocol(protocol)
	s.OpenMode = domain.ServiceOpenMode(openMode)

	var err error
	if s.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.PrivateService{}, err
	}
	if s.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.PrivateService{}, err
	}
	return s, nil
}
