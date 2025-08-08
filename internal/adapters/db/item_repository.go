package db

import (
	"context"
	"database/sql"
	"fmt"

	"troffee-auction-service/internal/domain/shared"

	"github.com/google/uuid"
)

// ItemRepository implements the item repository interface
type ItemRepository struct {
	conn *Connection
}

// NewItemRepository creates a new item repository
func NewItemRepository(conn *Connection) *ItemRepository {
	return &ItemRepository{conn: conn}
}

// Create creates a new item
func (r *ItemRepository) Create(ctx context.Context, item *shared.Item) error {
	query := `
		INSERT INTO items (id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.conn.GetDB().ExecContext(ctx, query,
		item.ID,
		item.Name,
		item.Description,
		item.CreatedAt,
		item.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create item: %w", err)
	}

	return nil
}

// GetByID retrieves an item by ID
func (r *ItemRepository) GetByID(ctx context.Context, id uuid.UUID) (*shared.Item, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM items
		WHERE id = $1
	`

	var item shared.Item
	err := r.conn.GetDB().QueryRowContext(ctx, query, id).Scan(
		&item.ID,
		&item.Name,
		&item.Description,
		&item.CreatedAt,
		&item.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrItemNotFound
		}
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	return &item, nil
}

// Update updates an item
func (r *ItemRepository) Update(ctx context.Context, item *shared.Item) error {
	query := `
		UPDATE items
		SET name = $2, description = $3, updated_at = $4
		WHERE id = $1
	`

	result, err := r.conn.GetDB().ExecContext(ctx, query,
		item.ID,
		item.Name,
		item.Description,
		item.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return shared.ErrItemNotFound
	}

	return nil
}

// Delete deletes an item
func (r *ItemRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM items WHERE id = $1`

	result, err := r.conn.GetDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return shared.ErrItemNotFound
	}

	return nil
}
