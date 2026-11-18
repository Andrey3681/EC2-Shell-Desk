package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ec2-shell-desk/internal/core/domain"
)

// GroupRepository implements ports.HostGroupRepository backed by SQLite with strict owner_user_id isolation.
type GroupRepository struct {
	db *sql.DB
}

// NewGroupRepository creates a GroupRepository using the provided SQLite Store.
func NewGroupRepository(store *Store) *GroupRepository {
	return &GroupRepository{db: store.db}
}

// Create inserts a new HostGroup owned by group.OwnerUserID.
func (r *GroupRepository) Create(ctx context.Context, group domain.HostGroup) error {
	const query = `
		INSERT INTO host_groups (id, owner_user_id, name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(
		ctx,
		query,
		group.ID,
		group.OwnerUserID,
		group.Name,
		formatTime(group.CreatedAt),
		formatTime(group.UpdatedAt),
	)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return fmt.Errorf("%w: group name %q already exists", domain.ErrDuplicateRecord, group.Name)
		}
		return fmt.Errorf("insert host group: %w", err)
	}
	return nil
}

// GetByID retrieves a HostGroup only if it belongs to ownerUserID.
func (r *GroupRepository) GetByID(ctx context.Context, ownerUserID, groupID string) (domain.HostGroup, error) {
	const query = `
		SELECT id, owner_user_id, name, created_at, updated_at
		FROM host_groups
		WHERE id = ? AND owner_user_id = ?`

	group, err := scanGroupRow(r.db.QueryRowContext(ctx, query, groupID, ownerUserID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.HostGroup{}, fmt.Errorf("%w: host group", domain.ErrNotFound)
		}
		return domain.HostGroup{}, err
	}
	return group, nil
}

// ListByUser returns all HostGroup records belonging strictly to ownerUserID.
func (r *GroupRepository) ListByUser(ctx context.Context, ownerUserID string) ([]domain.HostGroup, error) {
	const query = `
		SELECT id, owner_user_id, name, created_at, updated_at
		FROM host_groups
		WHERE owner_user_id = ?
		ORDER BY name COLLATE NOCASE ASC`

	rows, err := r.db.QueryContext(ctx, query, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list host groups: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	groups := make([]domain.HostGroup, 0)
	for rows.Next() {
		g, scanErr := scanGroupRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate host groups: %w", err)
	}
	return groups, nil
}

// Update modifies an existing HostGroup only if owned by group.OwnerUserID.
func (r *GroupRepository) Update(ctx context.Context, group domain.HostGroup) error {
	const query = `
		UPDATE host_groups
		SET name = ?, updated_at = ?
		WHERE id = ? AND owner_user_id = ?`

	res, err := r.db.ExecContext(
		ctx,
		query,
		group.Name,
		formatTime(group.UpdatedAt),
		group.ID,
		group.OwnerUserID,
	)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return fmt.Errorf("%w: group name %q already exists", domain.ErrDuplicateRecord, group.Name)
		}
		return fmt.Errorf("update host group: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated group rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: host group", domain.ErrNotFound)
	}
	return nil
}

// Delete removes a HostGroup owned by ownerUserID; member hosts become ungrouped automatically.
func (r *GroupRepository) Delete(ctx context.Context, ownerUserID, groupID string) error {
	const query = `DELETE FROM host_groups WHERE id = ? AND owner_user_id = ?`
	res, err := r.db.ExecContext(ctx, query, groupID, ownerUserID)
	if err != nil {
		return fmt.Errorf("delete host group: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted group rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: host group", domain.ErrNotFound)
	}
	return nil
}

func scanGroupRow(scanner rowScanner) (domain.HostGroup, error) {
	var (
		g         domain.HostGroup
		createdAt string
		updatedAt string
	)
	if err := scanner.Scan(&g.ID, &g.OwnerUserID, &g.Name, &createdAt, &updatedAt); err != nil {
		return domain.HostGroup{}, fmt.Errorf("scan host group row: %w", err)
	}
	var err error
	if g.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.HostGroup{}, err
	}
	if g.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.HostGroup{}, err
	}
	return g, nil
}
