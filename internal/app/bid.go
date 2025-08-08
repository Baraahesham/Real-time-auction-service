package app

import (
	"context"
	"time"

	"troffee-auction-service/internal/adapters/db"
	"troffee-auction-service/internal/domain/bid"
	"troffee-auction-service/internal/domain/shared"
	"troffee-auction-service/internal/ports/inbound"
	"troffee-auction-service/internal/ports/outbound"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// BidService implements the bid use cases
type BidService struct {
	bidRepo     outbound.BidRepository
	auctionRepo outbound.AuctionRepository
	userRepo    outbound.UserRepository
	broadcaster outbound.Broadcaster
	logger      zerolog.Logger
}

type BidServiceParams struct {
	BidRepo     outbound.BidRepository
	AuctionRepo outbound.AuctionRepository
	UserRepo    outbound.UserRepository
	Broadcaster outbound.Broadcaster
	Logger      zerolog.Logger
}

// NewBidService creates a new bid service
func NewBidService(params BidServiceParams) *BidService {
	return &BidService{
		bidRepo:     params.BidRepo,
		auctionRepo: params.AuctionRepo,
		userRepo:    params.UserRepo,
		broadcaster: params.Broadcaster,
		logger:      params.Logger.With().Str("component", "bid_service").Logger(),
	}
}

// PlaceBid places a new bid on an auction
func (client *BidService) PlaceBid(ctx context.Context, req inbound.PlaceBidRequest) (*bid.Bid, error) {
	client.logger.Info().
		Str("auction_id", req.AuctionID.String()).
		Str("user_id", req.UserID.String()).
		Float64("amount", req.Amount).
		Msg("Attempting to place bid")

	// Check if client is subscribed to the auction
	if client.broadcaster != nil {
		isSubscribed := client.broadcaster.IsSubscribed(ctx, req.AuctionID, req.ClientID)
		if !isSubscribed {
			client.logger.Warn().
				Str("client_id", req.ClientID).
				Str("user_id", req.UserID.String()).
				Str("auction_id", req.AuctionID.String()).
				Msg("Client not subscribed to auction")
			return nil, shared.ErrUserNotSubscribed
		}
	}

	// Validate auction exists and is active
	auction, err := client.auctionRepo.GetByID(ctx, req.AuctionID)
	if err != nil {
		client.logger.Error().Err(err).Str("auction_id", req.AuctionID.String()).Msg("Auction not found")
		return nil, shared.ErrAuctionNotFound
	}

	client.logger.Info().Interface("req", req).
		Msg("Auction details for bid validation")

	if !auction.CanBid() {
		client.logger.Warn().Str("auction_id", req.AuctionID.String()).Msg("Auction not accepting bids")
		return nil, shared.ErrAuctionNotAcceptingBids
	}
	if !auction.AuctionStarted() {
		client.logger.Warn().Str("auction_id", req.AuctionID.String()).Msg("Auction not started")
		return nil, shared.ErrAuctionNotStarted
	}

	// Validate user exists
	user, err := client.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		client.logger.Error().Err(err).Str("user_id", req.UserID.String()).Msg("User not found")
		return nil, shared.ErrUserNotFound
	}

	client.logger.Debug().Str("user_id", user.ID.String()).Str("name", user.Name).Msg("User validated")

	// Validate bid amount
	if req.Amount <= 0 {
		client.logger.Warn().Float64("amount", req.Amount).Msg("Invalid bid amount (must be > 0)")
		return nil, shared.ErrBidAmountInvalid
	}

	// Get current highest bid
	highestBid, err := client.bidRepo.GetHighestBid(ctx, req.AuctionID)
	if err != nil && err != shared.ErrNoBidsFound {
		client.logger.Error().Err(err).Str("auction_id", req.AuctionID.String()).Msg("Failed to get highest bid")
		return nil, err
	}

	// Validate bid is higher than current highest bid
	if highestBid != nil && req.Amount <= highestBid.Amount {
		client.logger.Warn().
			Str("auction_id", req.AuctionID.String()).
			Float64("current_highest_bid", highestBid.Amount).
			Float64("new_bid_amount", req.Amount).
			Msg("Bid amount too low (must be higher than current highest bid)")
		return nil, shared.ErrBidAmountTooLow
	}

	// Validate bid is higher than starting price if no previous bids
	if highestBid == nil && req.Amount <= auction.StartingPrice {
		client.logger.Warn().
			Str("auction_id", req.AuctionID.String()).
			Float64("starting_price", auction.StartingPrice).
			Float64("new_bid_amount", req.Amount).
			Msg("Bid amount below starting price")
		return nil, shared.ErrBidAmountBelowStarting
	}

	// Create new bid
	newBid := &bid.Bid{
		ID:        uuid.New(),
		AuctionID: req.AuctionID,
		UserID:    user.ID,
		Amount:    req.Amount,
		Status:    bid.StatusAccepted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	client.logger.Info().Interface("newBid", newBid).Msg("Created new bid object")

	// Use optimistic concurrency control for bid placement
	// This ensures strong consistency as required
	err = client.placeBidWithOCC(ctx, newBid, auction.CurrentPrice)
	if err != nil {
		client.logger.Error().Err(err).Str("bid_id", newBid.ID.String()).Msg("Failed to place bid with OCC")
		return nil, err
	}
	// Subscribe the user to the auction if not already subscribed
	if client.broadcaster != nil {
		clientID := newBid.UserID.String()
		eventChan := make(chan outbound.Event, 10) // buffered channel for events
		err := client.broadcaster.Subscribe(ctx, newBid.AuctionID, clientID, eventChan)
		if err != nil {
			// Only log if it's a real error (not "already subscribed")
			client.logger.Warn().
				Str("user_id", clientID).
				Str("auction_id", newBid.AuctionID.String()).
				Err(err).
				Msg("Failed to subscribe user to auction after successful bid")
		} else {
			client.logger.Info().
				Str("user_id", clientID).
				Str("auction_id", newBid.AuctionID.String()).
				Msg("User subscribed to auction after successful bid")
		}
	}
	// Broadcast the new bid
	event := outbound.Event{
		Type:      outbound.EventTypeBidPlaced,
		AuctionID: req.AuctionID,
		Data: map[string]interface{}{
			"bid_id":    newBid.ID,
			"user_id":   newBid.UserID,
			"amount":    newBid.Amount,
			"timestamp": newBid.CreatedAt.Unix(),
		},
		Timestamp: newBid.CreatedAt.Unix(),
	}

	if err := client.broadcaster.Publish(ctx, req.AuctionID, event); err != nil {
		// Log error but don't fail the bid placement
		client.logger.Error().Err(err).Str("bid_id", newBid.ID.String()).Msg("Failed to broadcast bid event")
	} else {
		client.logger.Info().
			Str("bid_id", newBid.ID.String()).
			Str("auction_id", newBid.AuctionID.String()).
			Str("user_id", newBid.UserID.String()).
			Float64("amount", newBid.Amount).
			Msg("Bid placed successfully and broadcasted")
	}

	return newBid, nil
}

// placeBidWithOCC places a bid using optimistic concurrency control
func (s *BidService) placeBidWithOCC(ctx context.Context, newBid *bid.Bid, currentPrice float64) error {
	s.logger.Debug().
		Str("bid_id", newBid.ID.String()).
		Float64("current_price", currentPrice).
		Msg("Attempting to place bid with OCC")

	// Cast the repository to access the OCC method
	bidRepo, ok := s.bidRepo.(*db.BidRepository)
	if !ok {
		s.logger.Warn().Msg("Repository doesn't support OCC, using fallback approach")
		// Fallback to simple approach if repository doesn't support OCC
		if err := s.bidRepo.Create(ctx, newBid); err != nil {
			s.logger.Error().Err(err).Str("bid_id", newBid.ID.String()).Msg("Failed to create bid in fallback mode")
			return err
		}

		// Mark bid as accepted
		newBid.Accept()
		if err := s.bidRepo.Update(ctx, newBid); err != nil {
			s.logger.Error().Err(err).Str("bid_id", newBid.ID.String()).Msg("Failed to update bid status in fallback mode")
			return err
		}
		s.logger.Info().Str("bid_id", newBid.ID.String()).Msg("Bid placed successfully using fallback approach")
		return nil
	}

	// Use the repository's OCC method
	s.logger.Debug().Str("bid_id", newBid.ID.String()).Msg("Using OCC method for bid placement")
	if err := bidRepo.PlaceBidWithOCC(ctx, newBid, currentPrice); err != nil {
		s.logger.Error().Err(err).Str("bid_id", newBid.ID.String()).Msg("Failed to place bid with OCC")
		return err
	}
	s.logger.Info().Str("bid_id", newBid.ID.String()).Msg("Bid placed successfully using OCC")
	return nil
}

// GetBids retrieves bids for an auction
func (s *BidService) GetBids(ctx context.Context, auctionID uuid.UUID) ([]*bid.Bid, error) {
	return s.bidRepo.GetByAuctionID(ctx, auctionID)
}

// GetHighestBid retrieves the highest bid for an auction
func (s *BidService) GetHighestBid(ctx context.Context, auctionID uuid.UUID) (*bid.Bid, error) {
	return s.bidRepo.GetHighestBid(ctx, auctionID)
}
