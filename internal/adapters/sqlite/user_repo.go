package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"ec2-shell-desk/internal/core/domain"
)

// UserRepository implements ports.UserRepository backed by SQLite.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a UserRepository using the provided SQLite Store.
func NewUserRepository(store *Store) *UserRepository {
	return &UserRepository{db: store.db}
}

// Create inserts a new local user into SQLite.
func (r *UserRepository) Create(ctx context.Context, user domain.User) error {
	const query = `
		INSERT INTO users (id, username, password_hash, auth_salt, vault_salt, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	isDefault := 0
	if user.IsDefault {
		isDefault = 1
	}

	_, err := r.db.ExecContext(
		ctx,
		query,
		user.ID,
		user.Username,
		user.PasswordHash,
		user.AuthSalt,
		user.VaultSalt,
		isDefault,
		formatTime(user.CreatedAt),
		formatTime(user.UpdatedAt),
	)
	if err != nil {
		if isUniqueConstraintErr(err) {
			return fmt.Errorf("%w: username %q already exists", domain.ErrUserAlreadyExists, user.Username)
		}
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

// GetByID retrieves a local user by primary ID.
func (r *UserRepository) GetByID(ctx context.Context, userID string) (domain.User, error) {
	const query = `
		SELECT id, username, password_hash, auth_salt, vault_salt, is_default, created_at, updated_at
		FROM users
		WHERE id = ?`
	return r.scanSingleUser(r.db.QueryRowContext(ctx, query, userID))
}

// GetByUsername retrieves a local user by username (case-insensitive).
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (domain.User, error) {
	const query = `
		SELECT id, username, password_hash, auth_salt, vault_salt, is_default, created_at, updated_at
		FROM users
		WHERE username = ?`
	return r.scanSingleUser(r.db.QueryRowContext(ctx, query, username))
}

// List returns all registered local users ordered by creation time.
func (r *UserRepository) List(ctx context.Context) ([]domain.User, error) {
	const query = `
		SELECT id, username, password_hash, auth_salt, vault_salt, is_default, created_at, updated_at
		FROM users
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	users := make([]domain.User, 0)
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

// Count returns the total number of registered local users.
func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM users`).Scan(&total); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return total, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (r *UserRepository) scanSingleUser(row *sql.Row) (domain.User, error) {
	u, err := scanUserRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, fmt.Errorf("%w: user", domain.ErrNotFound)
		}
		return domain.User{}, err
	}
	return u, nil
}

func scanUserRow(scanner rowScanner) (domain.User, error) {
	var (
		u         domain.User
		isDefault int
		createdAt string
		updatedAt string
	)
	if err := scanner.Scan(
		&u.ID,
		&u.Username,
		&u.PasswordHash,
		&u.AuthSalt,
		&u.VaultSalt,
		&isDefault,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.User{}, fmt.Errorf("scan user row: %w", err)
	}
	u.IsDefault = isDefault == 1

	var err error
	if u.CreatedAt, err = parseTime(createdAt); err != nil {
		return domain.User{}, err
	}
	if u.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return domain.User{}, err
	}
	return u, nil
}
