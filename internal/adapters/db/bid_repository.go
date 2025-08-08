package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"troffee-auction-service/internal/domain/bid"
	"troffee-auction-service/internal/domain/shared"

	"github.com/google/uuid"
)

// BidRepository implements the bid repository interface
type BidRepository struct {
	conn *Connection
}

// NewBidRepository creates a new bid repository
func NewBidRepository(conn *Connection) *BidRepository {
	return &BidRepository{conn: conn}
}

func (r *BidRepository) Create(ctx context.Context, bid *bid.Bid) error {
	query := `
		INSERT INTO bids (id, auction_id, user_id, amount, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.conn.GetDB().ExecContext(ctx, query,
		bid.ID,
		bid.AuctionID,
		bid.UserID,
		bid.Amount,
		bid.Status,
		bid.CreatedAt,
		bid.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create bid: %w", err)
	}

	return nil
}

func (r *BidRepository) GetByID(ctx context.Context, id uuid.UUID) (*bid.Bid, error) {
	query := `
		SELECT id, auction_id, user_id, amount, status, created_at, updated_at
		FROM bids
		WHERE id = $1
	`

	var bid bid.Bid
	err := r.conn.GetDB().QueryRowContext(ctx, query, id).Scan(
		&bid.ID,
		&bid.AuctionID,
		&bid.UserID,
		&bid.Amount,
		&bid.Status,
		&bid.CreatedAt,
		&bid.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("bid not found")
		}
		return nil, fmt.Errorf("failed to get bid: %w", err)
	}

	return &bid, nil
}

// GetByAuctionID retrieves all bids for an auction
func (r *BidRepository) GetByAuctionID(ctx context.Context, auctionID uuid.UUID) ([]*bid.Bid, error) {
	query := `
		SELECT id, auction_id, user_id, amount, status, created_at, updated_at
		FROM bids
		WHERE auction_id = $1
		ORDER BY amount DESC, created_at ASC
	`

	rows, err := r.conn.GetDB().QueryContext(ctx, query, auctionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bids: %w", err)
	}
	defer rows.Close()

	var bids []*bid.Bid
	for rows.Next() {
		var bid bid.Bid
		err := rows.Scan(
			&bid.ID,
			&bid.AuctionID,
			&bid.UserID,
			&bid.Amount,
			&bid.Status,
			&bid.CreatedAt,
			&bid.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan bid: %w", err)
		}
		bids = append(bids, &bid)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bids: %w", err)
	}

	return bids, nil
}

// GetHighestBid retrieves the highest bid for an auction
func (r *BidRepository) GetHighestBid(ctx context.Context, auctionID uuid.UUID) (*bid.Bid, error) {
	query := `
		SELECT id, auction_id, user_id, amount, status, created_at, updated_at
		FROM bids
		WHERE auction_id = $1 AND status = 'accepted'
		ORDER BY amount DESC, created_at ASC
		LIMIT 1
	`

	var bid bid.Bid
	err := r.conn.GetDB().QueryRowContext(ctx, query, auctionID).Scan(
		&bid.ID,
		&bid.AuctionID,
		&bid.UserID,
		&bid.Amount,
		&bid.Status,
		&bid.CreatedAt,
		&bid.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, shared.ErrNoBidsFound
		}
		return nil, fmt.Errorf("failed to get highest bid: %w", err)
	}

	return &bid, nil
}

// Update updates a bid
func (r *BidRepository) Update(ctx context.Context, bid *bid.Bid) error {
	query := `
		UPDATE bids
		SET auction_id = $2, user_id = $3, amount = $4, status = $5, updated_at = $6
		WHERE id = $1
	`

	result, err := r.conn.GetDB().ExecContext(ctx, query,
		bid.ID,
		bid.AuctionID,
		bid.UserID,
		bid.Amount,
		bid.Status,
		bid.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update bid: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("bid not found")
	}

	return nil
}

/*
PlaceBidWithOCC places a bid using optimistic concurrency control.
 1. Reading the current auction state
 2. Validating the expected price matches the actual price
 3. Updating the auction only if the price hasn't changed
 4. Failing if another transaction modified the auction concurrently
*/
func (r *BidRepository) PlaceBidWithOCC(ctx context.Context, newBid *bid.Bid, expectedCurrentPrice float64) error {
	return r.conn.ExecuteTransaction(func(tx *sql.Tx) error {
		// First, check if the auction is still active
		auctionQuery := `
			SELECT current_price, status, updated_at
			FROM auctions
			WHERE id = $1
		`

		var dbCurrentPrice float64
		var status string
		var updatedAt time.Time
		err := tx.QueryRowContext(ctx, auctionQuery, newBid.AuctionID).Scan(&dbCurrentPrice, &status, &updatedAt)
		if err != nil {
			if err == sql.ErrNoRows {
				return shared.ErrAuctionNotFound
			}
			return fmt.Errorf("failed to get auction for OCC: %w", err)
		}

		if status != "active" {
			return shared.ErrAuctionNotAcceptingBids
		}

		if dbCurrentPrice != expectedCurrentPrice {
			return shared.ErrBidAmountTooLow
		}

		if newBid.Amount <= dbCurrentPrice {
			return shared.ErrBidAmountTooLow
		}

		// Insert the new bid
		bidQuery := `
			INSERT INTO bids (id, auction_id, user_id, amount, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`

		_, err = tx.ExecContext(ctx, bidQuery,
			newBid.ID,
			newBid.AuctionID,
			newBid.UserID,
			newBid.Amount,
			newBid.Status,
			newBid.CreatedAt,
			newBid.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert bid: %w", err)
		}

		// Use the expected current price in the WHERE clause to ensure no other transaction modified it
		updateQuery := `
			UPDATE auctions
			SET current_price = $2, updated_at = $3
			WHERE id = $1 AND current_price = $4
		`

		result, err := tx.ExecContext(ctx, updateQuery,
			newBid.AuctionID,
			newBid.Amount,
			newBid.CreatedAt,
			expectedCurrentPrice,
		)
		if err != nil {
			return fmt.Errorf("failed to update auction price: %w", err)
		}

		// Check if the update actually affected a row
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}

		// If no rows were affected, it means another transaction modified the auction
		if rowsAffected == 0 {
			return shared.ErrBidAmountTooLow
		}

		return nil
	})
}
