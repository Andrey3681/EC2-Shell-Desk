package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ec2-shell-desk/internal/core/domain"
)

// SnippetRepository implements ports.SnippetRepository backed by SQLite with strict owner_user_id isolation.
type SnippetRepository struct {
	db *sql.DB
}

// NewSnippetRepository creates a SnippetRepository using the provided SQLite Store.
func NewSnippetRepository(store *Store) *SnippetRepository {
	return &SnippetRepository{db: store.db}
}

// Create inserts a new CommandSnippet owned by snippet.OwnerUserID.
func (r *SnippetRepository) Create(ctx context.Context, snippet domain.CommandSnippet) error {
	const query = `
		INSERT INTO command_snippets (
			id, owner_user_id, host_id, group_id, name, command, auto_execute, notes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var hostID sql.NullString
	if snippet.HostID != "" {
		hostID = sql.NullString{String: snippet.HostID, Valid: true}
	}
	var groupID sql.NullString
	if snippet.GroupID != "" {
		groupID = sql.NullString{String: snippet.GroupID, Valid: true}
	}
	autoExec := 0
	if snippet.AutoExecute {
		autoExec = 1
	}

	_, err := r.db.ExecContext(
		ctx,
		query,
		snippet.ID,
		snippet.OwnerUserID,
		hostID,
		groupID,
		snippet.Name,
		snippet.Command,
		autoExec,
		snippet.Notes,
		formatTime(snippet.CreatedAt),
		formatTime(snippet.UpdatedAt),
	)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return fmt.Errorf("%w: snippet name %q already exists", domain.ErrDuplicateRecord, snippet.Name)
		}
		return fmt.Errorf("insert command snippet: %w", err)
	}
	return nil
}

// GetByID retrieves a CommandSnippet only if it belongs to ownerUserID.
func (r *SnippetRepository) GetByID(ctx context.Context, ownerUserID, snippetID string) (domain.CommandSnippet, error) {
	const query = `
		SELECT id, owner_user_id, host_id, group_id, name, command, auto_execute, notes, created_at, updated_at
		FROM command_snippets
		WHERE id = ? AND owner_user_id = ?`

	snippet, err := scanSnippetRow(r.db.QueryRowContext(ctx, query, snippetID, ownerUserID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.CommandSnippet{}, fmt.Errorf("%w: command snippet", domain.ErrNotFound)
		}
		return domain.CommandSnippet{}, err
	}
	return snippet, nil
}

// ListByUser returns all CommandSnippet records belonging strictly to ownerUserID.
func (r *SnippetRepository) ListByUser(ctx context.Context, ownerUserID string) ([]domain.CommandSnippet, error) {
	const query = `
		SELECT id, owner_user_id, host_id, group_id, name, command, auto_execute, notes, created_at, updated_at
		FROM command_snippets
		WHERE owner_user_id = ?
		ORDER BY name COLLATE NOCASE ASC`

	rows, err := r.db.QueryContext(ctx, query, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("list command snippets: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	snippets := make([]domain.CommandSnippet, 0)
	for rows.Next() {
		s, scanErr := scanSnippetRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		snippets = append(snippets, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate command snippets: %w", err)
	}
	return snippets, nil
}

// Update modifies an existing CommandSnippet only if owned by snippet.OwnerUserID.
func (r *SnippetRepository) Update(ctx context.Context, snippet domain.CommandSnippet) error {
	const query = `
		UPDATE command_snippets
		SET host_id = ?, group_id = ?, name = ?, command = ?, auto_execute = ?, notes = ?, updated_at = ?
		WHERE id = ? AND owner_user_id = ?`

	var hostID sql.NullString
	if snippet.HostID != "" {
		hostID = sql.NullString{String: snippet.HostID, Valid: true}
	}
	var groupID sql.NullString
	if snippet.GroupID != "" {
		groupID = sql.NullString{String: snippet.GroupID, Valid: true}
	}
	autoExec := 0
	if snippet.AutoExecute {
		autoExec = 1
	}

	res, err := r.db.ExecContext(
		ctx,
		query,
		hostID,
		groupID,
		snippet.Name,
		snippet.Command,
		autoExec,
		snippet.Notes,
		formatTime(snippet.UpdatedAt),
		snippet.ID,
		snippet.OwnerUserID,
	)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return fmt.Errorf("%w: snippet name %q already exists", domain.ErrDuplicateRecord, snippet.Name)
		}
		return fmt.Errorf("update command snippet: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check updated snippet rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: command snippet", domain.ErrNotFound)
	}
	return nil
}

// Delete removes a CommandSnippet only if owned by ownerUserID.
func (r *SnippetRepository) Delete(ctx context.Context, ownerUserID, snippetID string) error {
	const query = `DELETE FROM command_snippets WHERE id = ? AND owner_user_id = ?`
	res, err := r.db.ExecContext(ctx, query, snippetID, ownerUserID)
	if err != nil {
		return fmt.Errorf("delete command snippet: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted snippet rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: command snippet", domain.ErrNotFound)
	}
	return nil
}

func scanSnippetRow(scanner rowScanner) (domain.CommandSnippet, error) {
	var (
		s         domain.CommandSnippet
		hostID    sql.NullString
		groupID   sql.NullString
		autoExec  int
		createdAt string
		updatedAt string
	)
	if err := scanner.Scan(
		&s.ID,
		&s.OwnerUserID,
		&hostID,
		&groupID,
		&s.Name,
		&s.Command,
		&autoExec,
		&s.Notes,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.CommandSnippet{}, fmt.Errorf("scan command snippet row: %w", err)
	}
	if hostID.Valid {
		s.HostID = hostID.String
	}
	if groupID.Valid {
		s.GroupID = groupID.String
	}
	s.AutoExecute = autoExec == 1

	var err error
	if s.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.CommandSnippet{}, err
	}
	if s.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.CommandSnippet{}, err
	}
	return s, nil
}
