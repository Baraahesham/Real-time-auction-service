package outbound

import (
	"context"

	"troffee-auction-service/internal/domain/auction"
	"troffee-auction-service/internal/domain/bid"
	"troffee-auction-service/internal/domain/shared"

	"github.com/google/uuid"
)

// AuctionRepository defines the interface for auction data operations
type AuctionRepository interface {
	// Create creates a new auction
	Create(ctx context.Context, auction *auction.Auction) error

	// GetByID retrieves an auction by ID
	GetByID(ctx context.Context, id uuid.UUID) (*auction.Auction, error)

	// List retrieves a list of auctions with optional filters
	List(ctx context.Context, status *auction.Status, page, pageSize int) ([]*auction.Auction, error)

	// GetActiveByItemID retrieves active auctions for a specific item
	GetActiveByItemID(ctx context.Context, itemID uuid.UUID) ([]*auction.Auction, error)

	// Update updates an auction
	Update(ctx context.Context, auction *auction.Auction) error

	// Delete deletes an auction
	Delete(ctx context.Context, id uuid.UUID) error
}

// BidRepository defines the interface for bid data operations
type BidRepository interface {
	// Create creates a new bid
	Create(ctx context.Context, bid *bid.Bid) error

	// GetByID retrieves a bid by ID
	GetByID(ctx context.Context, id uuid.UUID) (*bid.Bid, error)

	// GetByAuctionID retrieves all bids for an auction
	GetByAuctionID(ctx context.Context, auctionID uuid.UUID) ([]*bid.Bid, error)

	// GetHighestBid retrieves the highest bid for an auction
	GetHighestBid(ctx context.Context, auctionID uuid.UUID) (*bid.Bid, error)

	// Update updates a bid
	Update(ctx context.Context, bid *bid.Bid) error

	// PlaceBidWithOCC places a bid using optimistic concurrency control
	PlaceBidWithOCC(ctx context.Context, bid *bid.Bid, expectedCurrentPrice float64) error
}

// ItemRepository defines the interface for item data operations
type ItemRepository interface {
	// Create creates a new item
	Create(ctx context.Context, item *shared.Item) error

	// GetByID retrieves an item by ID
	GetByID(ctx context.Context, id uuid.UUID) (*shared.Item, error)

	// Update updates an item
	Update(ctx context.Context, item *shared.Item) error

	// Delete deletes an item
	Delete(ctx context.Context, id uuid.UUID) error
}

// UserRepository defines the interface for user data operations
type UserRepository interface {
	// GetByID retrieves a user by ID
	GetByID(ctx context.Context, id uuid.UUID) (*shared.User, error)

	// Create creates a new user
	Create(ctx context.Context, user *shared.User) error
}
