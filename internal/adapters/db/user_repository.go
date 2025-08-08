package db

import (
	"context"
	"database/sql"
	"fmt"

	"troffee-auction-service/internal/domain/shared"

	"github.com/google/uuid"
)

// UserRepository implements the user repository interface
type UserRepository struct {
	conn *Connection
}

// NewUserRepository creates a new user repository
func NewUserRepository(conn *Connection) *UserRepository {
	return &UserRepository{conn: conn}
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*shared.User, error) {
	query := `
		SELECT id, name
		FROM users
		WHERE id = $1
	`

	var user shared.User
	err := r.conn.GetDB().QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Name,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, user *shared.User) error {
	query := `
		INSERT INTO users (id, name)
		VALUES ($1, $2)
	`

	_, err := r.conn.GetDB().ExecContext(ctx, query,
		user.ID,
		user.Name,
	)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}
