package inbound

import (
	"context"

	"troffee-auction-service/internal/domain/auction"
	"troffee-auction-service/internal/domain/bid"

	"github.com/google/uuid"
)

// AuctionService defines the interface for auction operations
type AuctionService interface {
	// CreateAuction creates a new auction
	CreateAuction(ctx context.Context, req CreateAuctionRequest) (*auction.Auction, error)

	// GetAuction retrieves an auction by ID
	GetAuction(ctx context.Context, auctionID uuid.UUID) (*auction.Auction, error)

	// ListAuctions retrieves a list of auctions
	ListAuctions(ctx context.Context, req ListAuctionsRequest) ([]*auction.Auction, error)

	// EndAuction ends an auction
	EndAuction(ctx context.Context, auctionID uuid.UUID) error
}

// BidService defines the interface for bid operations
type BidService interface {
	// PlaceBid places a new bid on an auction
	PlaceBid(ctx context.Context, req PlaceBidRequest) (*bid.Bid, error)

	// GetBids retrieves bids for an auction
	GetBids(ctx context.Context, auctionID uuid.UUID) ([]*bid.Bid, error)

	// GetHighestBid retrieves the highest bid for an auction
	GetHighestBid(ctx context.Context, auctionID uuid.UUID) (*bid.Bid, error)
}

// request to create an auction
type CreateAuctionRequest struct {
	ItemID        uuid.UUID `json:"item_id"`
	CreatorID     uuid.UUID `json:"creator_id"`
	StartTime     string    `json:"start_time"`
	EndTime       string    `json:"end_time"`
	StartingPrice float64   `json:"starting_price"`
}

// request to list auctions
type ListAuctionsRequest struct {
	Status   *auction.Status `json:"status,omitempty"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// request to place a bid
type PlaceBidRequest struct {
	AuctionID uuid.UUID `json:"auction_id"`
	UserID    uuid.UUID `json:"user_id"`
	ClientID  string    `json:"client_id"`
	Amount    float64   `json:"amount"`
}
