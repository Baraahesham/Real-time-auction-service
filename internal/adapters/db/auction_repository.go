package db

import (
	"context"
	"database/sql"
	"fmt"

	"troffee-auction-service/internal/domain/auction"
	"troffee-auction-service/internal/domain/shared"

	"github.com/google/uuid"
)

// AuctionRepository implements the auction repository interface
type AuctionRepository struct {
	conn *Connection
}

// NewAuctionRepository creates a new auction repository
func NewAuctionRepository(conn *Connection) *AuctionRepository {
	return &AuctionRepository{conn: conn}
}

// Create creates a new auction
func (r *AuctionRepository) Create(ctx context.Context, auction *auction.Auction) error {
	query := `
		INSERT INTO auctions (id, item_id, creator_id, start_time, end_time, starting_price, current_price, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.conn.GetDB().ExecContext(ctx, query,
		auction.ID,
		auction.ItemID,
		auction.CreatorID,
		auction.StartTime,
		auction.EndTime,
		auction.StartingPrice,
		auction.CurrentPrice,
		auction.Status,
		auction.CreatedAt,
		auction.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create auction: %w", err)
	}

	return nil
}

// GetByID retrieves an auction by ID
func (r *AuctionRepository) GetByID(ctx context.Context, id uuid.UUID) (*auction.Auction, error) {
	query := `
		SELECT id, item_id, creator_id, start_time, end_time, starting_price, current_price, status, created_at, updated_at
		FROM auctions
		WHERE id = $1
	`

	var auction auction.Auction
	err := r.conn.GetDB().QueryRowContext(ctx, query, id).Scan(
		&auction.ID,
		&auction.ItemID,
		&auction.CreatorID,
		&auction.StartTime,
		&auction.EndTime,
		&auction.StartingPrice,
		&auction.CurrentPrice,
		&auction.Status,
		&auction.CreatedAt,
		&auction.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrAuctionNotFound
		}
		return nil, fmt.Errorf("failed to get auction: %w", err)
	}

	return &auction, nil
}

// List retrieves a list of auctions with optional filters
func (r *AuctionRepository) List(ctx context.Context, status *auction.Status, page, pageSize int) ([]*auction.Auction, error) {
	baseQuery := `
		SELECT id, item_id, creator_id, start_time, end_time, starting_price, current_price, status, created_at, updated_at
		FROM auctions
	`

	var whereClause string
	var args []interface{}
	argCount := 1

	if status != nil {
		whereClause = "WHERE status = $1"
		args = append(args, *status)
		argCount++
	}

	// Add pagination
	limitClause := fmt.Sprintf("LIMIT $%d", argCount)
	offsetClause := fmt.Sprintf("OFFSET $%d", argCount+1)
	args = append(args, pageSize, (page-1)*pageSize)

	query := baseQuery + whereClause + " ORDER BY created_at DESC " + limitClause + " " + offsetClause

	rows, err := r.conn.GetDB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list auctions: %w", err)
	}
	defer rows.Close()

	var auctions []*auction.Auction
	for rows.Next() {
		var auction auction.Auction
		err := rows.Scan(
			&auction.ID,
			&auction.ItemID,
			&auction.CreatorID,
			&auction.StartTime,
			&auction.EndTime,
			&auction.StartingPrice,
			&auction.CurrentPrice,
			&auction.Status,
			&auction.CreatedAt,
			&auction.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan auction: %w", err)
		}
		auctions = append(auctions, &auction)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating auctions: %w", err)
	}

	return auctions, nil
}

// GetActiveByItemID retrieves active auctions for a specific item
func (r *AuctionRepository) GetActiveByItemID(ctx context.Context, itemID uuid.UUID) ([]*auction.Auction, error) {
	query := `
		SELECT id, item_id, creator_id, start_time, end_time, starting_price, current_price, status, created_at, updated_at
		FROM auctions
		WHERE item_id = $1 AND status = 'active'
		ORDER BY created_at DESC
	`

	rows, err := r.conn.GetDB().QueryContext(ctx, query, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active auctions by item ID: %w", err)
	}
	defer rows.Close()

	var auctions []*auction.Auction
	for rows.Next() {
		var auction auction.Auction
		err := rows.Scan(
			&auction.ID,
			&auction.ItemID,
			&auction.CreatorID,
			&auction.StartTime,
			&auction.EndTime,
			&auction.StartingPrice,
			&auction.CurrentPrice,
			&auction.Status,
			&auction.CreatedAt,
			&auction.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan auction: %w", err)
		}
		auctions = append(auctions, &auction)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating auctions: %w", err)
	}

	return auctions, nil
}

// Update updates an auction
func (r *AuctionRepository) Update(ctx context.Context, auction *auction.Auction) error {
	query := `
		UPDATE auctions
		SET item_id = $2, creator_id = $3, start_time = $4, end_time = $5, 
		    starting_price = $6, current_price = $7, status = $8, updated_at = $9
		WHERE id = $1
	`

	result, err := r.conn.GetDB().ExecContext(ctx, query,
		auction.ID,
		auction.ItemID,
		auction.CreatorID,
		auction.StartTime,
		auction.EndTime,
		auction.StartingPrice,
		auction.CurrentPrice,
		auction.Status,
		auction.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update auction: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return shared.ErrAuctionNotFound
	}

	return nil
}

// Delete deletes an auction
func (r *AuctionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM auctions WHERE id = $1`

	result, err := r.conn.GetDB().ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete auction: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return shared.ErrAuctionNotFound
	}

	return nil
}
