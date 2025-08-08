package app

import (
	"context"
	"time"

	"troffee-auction-service/internal/adapters/scheduler"
	"troffee-auction-service/internal/domain/auction"
	"troffee-auction-service/internal/domain/shared"
	"troffee-auction-service/internal/ports/inbound"
	"troffee-auction-service/internal/ports/outbound"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// AuctionService implements the auction use cases and scheduler.AuctionEndService
type AuctionService struct {
	auctionRepo outbound.AuctionRepository
	itemRepo    outbound.ItemRepository
	userRepo    outbound.UserRepository
	bidRepo     outbound.BidRepository
	scheduler   *scheduler.AuctionScheduler
	logger      zerolog.Logger
}
type AuctionServiceParams struct {
	AuctionRepo outbound.AuctionRepository
	ItemRepo    outbound.ItemRepository
	UserRepo    outbound.UserRepository
	BidRepo     outbound.BidRepository
	Scheduler   *scheduler.AuctionScheduler
	Logger      zerolog.Logger
}

// NewAuctionService creates a new auction service
func NewAuctionService(params AuctionServiceParams) *AuctionService {
	return &AuctionService{
		auctionRepo: params.AuctionRepo,
		itemRepo:    params.ItemRepo,
		userRepo:    params.UserRepo,
		bidRepo:     params.BidRepo,
		scheduler:   params.Scheduler,
		logger:      params.Logger.With().Str("component", "auction_service").Logger(),
	}
}

// CreateAuction creates a new auction
func (service *AuctionService) CreateAuction(ctx context.Context, req inbound.CreateAuctionRequest) (*auction.Auction, error) {
	service.logger.Info().
		Str("item_id", req.ItemID.String()).
		Str("creator_id", req.CreatorID.String()).
		Str("start_time", req.StartTime).
		Str("end_time", req.EndTime).
		Float64("starting_price", req.StartingPrice).
		Msg("Attempting to create auction")

	// Validate item exists
	item, err := service.itemRepo.GetByID(ctx, req.ItemID)
	if err != nil {
		service.logger.Error().Err(err).Str("item_id", req.ItemID.String()).Msg("Item not found")
		return nil, shared.ErrItemNotFound
	}

	service.logger.Debug().
		Str("item_id", item.ID.String()).
		Str("item_name", item.Name).
		Msg("Item validated")

	// Validate user exists
	user, err := service.userRepo.GetByID(ctx, req.CreatorID)
	if err != nil {
		service.logger.Error().Err(err).Str("creator_id", req.CreatorID.String()).Msg("User not found")
		return nil, shared.ErrUserNotFound
	}

	service.logger.Debug().
		Str("user_id", user.ID.String()).
		Str("user_name", user.Name).
		Msg("User validated")

	// Parse times
	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		service.logger.Error().Err(err).Str("start_time", req.StartTime).Msg("Invalid start time format")
		return nil, shared.ErrInvalidTimeFormat
	}

	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		service.logger.Error().Err(err).Str("end_time", req.EndTime).Msg("Invalid end time format")
		return nil, shared.ErrInvalidTimeFormat
	}

	service.logger.Debug().
		Time("parsed_start_time", startTime).
		Time("parsed_end_time", endTime).
		Msg("Time parsing completed")

	// Validate time constraints
	now := time.Now()
	service.logger.Debug().
		Time("current_time", now).
		Time("start_time", startTime).
		Time("end_time", endTime).
		Msg("Validating time constraints")

	if startTime.Before(now) {
		service.logger.Warn().
			Time("start_time", startTime).
			Time("current_time", now).
			Msg("Start time cannot be in the past")
		return nil, shared.ErrInvalidStartTime
	}

	if endTime.Before(startTime) {
		service.logger.Warn().
			Time("start_time", startTime).
			Time("end_time", endTime).
			Msg("End time cannot be before start time")
		return nil, shared.ErrInvalidEndTime
	}

	if req.StartingPrice <= 0 {
		service.logger.Warn().Float64("starting_price", req.StartingPrice).Msg("Starting price must be greater than 0")
		return nil, shared.ErrInvalidStartingPrice
	}

	// Check if item is already in an active auction
	activeAuctions, err := service.auctionRepo.GetActiveByItemID(ctx, req.ItemID)
	if err != nil {
		service.logger.Error().Err(err).Str("item_id", req.ItemID.String()).Msg("Failed to check for active auctions")
		return nil, err
	}

	if len(activeAuctions) > 0 {
		service.logger.Warn().
			Str("item_id", req.ItemID.String()).
			Int("active_auctions_count", len(activeAuctions)).
			Msg("Item is already in an active auction")
		return nil, shared.ErrItemAlreadyInAuction
	}

	// Create auction
	auction := &auction.Auction{
		ID:            uuid.New(),
		ItemID:        item.ID,
		CreatorID:     user.ID,
		StartTime:     startTime,
		EndTime:       endTime,
		StartingPrice: req.StartingPrice,
		CurrentPrice:  req.StartingPrice,
		Status:        auction.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	service.logger.Info().
		Str("auction_id", auction.ID.String()).
		Str("item_id", auction.ItemID.String()).
		Str("creator_id", auction.CreatorID.String()).
		Time("start_time", auction.StartTime).
		Time("end_time", auction.EndTime).
		Float64("starting_price", auction.StartingPrice).
		Msg("Created auction object")

	// Save to database
	if err := service.auctionRepo.Create(ctx, auction); err != nil {
		service.logger.Error().Err(err).Str("auction_id", auction.ID.String()).Msg("Failed to save auction to database")
		return nil, err
	}

	service.logger.Info().
		Str("auction_id", auction.ID.String()).
		Msg("Auction created successfully")

	// Schedule auction for expiration
	if service.scheduler != nil {
		if err := service.scheduler.ScheduleAuction(auction.ID, auction.EndTime); err != nil {
			service.logger.Error().Err(err).Str("auction_id", auction.ID.String()).Msg("Failed to schedule auction for expiration")
			// Don't fail the auction creation, just log the error
		} else {
			service.logger.Info().
				Str("auction_id", auction.ID.String()).
				Time("end_time", auction.EndTime).
				Msg("Auction scheduled for expiration")
		}
	}

	return auction, nil
}

// GetAuction retrieves an auction by ID
func (client *AuctionService) GetAuction(ctx context.Context, auctionID uuid.UUID) (*auction.Auction, error) {
	client.logger.Debug().Str("auction_id", auctionID.String()).Msg("Retrieving auction")

	auction, err := client.auctionRepo.GetByID(ctx, auctionID)
	if err != nil {
		client.logger.Error().Err(err).Str("auction_id", auctionID.String()).Msg("Failed to retrieve auction")
		return nil, err
	}

	client.logger.Debug().
		Str("auction_id", auction.ID.String()).
		Str("auction_status", string(auction.Status)).
		Time("start_time", auction.StartTime).
		Time("end_time", auction.EndTime).
		Bool("can_bid", auction.CanBid()).
		Msg("Auction retrieved successfully")

	return auction, nil
}

// ListAuctions retrieves a list of auctions
func (client *AuctionService) ListAuctions(ctx context.Context, req inbound.ListAuctionsRequest) ([]*auction.Auction, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	return client.auctionRepo.List(ctx, req.Status, req.Page, req.PageSize)
}

// EndAuction ends an auction (implements inbound.AuctionService interface)
func (client *AuctionService) EndAuction(ctx context.Context, auctionID uuid.UUID) error {
	_, err := client.endAuctionWithResult(ctx, auctionID)
	return err
}

// endAuctionWithResult ends an auction and returns the result (for scheduler use)
func (client *AuctionService) endAuctionWithResult(ctx context.Context, auctionID uuid.UUID) (*shared.AuctionEndResult, error) {
	client.logger.Info().Str("auction_id", auctionID.String()).Msg("Ending auction")

	auction, err := client.auctionRepo.GetByID(ctx, auctionID)
	if err != nil {
		client.logger.Error().Err(err).Str("auction_id", auctionID.String()).Msg("Failed to retrieve auction for ending")
		return nil, err
	}

	if auction.IsEnded() {
		client.logger.Warn().Str("auction_id", auctionID.String()).Msg("Auction already ended")
		return nil, shared.ErrAuctionAlreadyEnded
	}

	auction.EndAuction()
	// Get the highest bid to determine winner
	highestBid, err := client.bidRepo.GetHighestBid(ctx, auctionID)
	if err != nil {
		client.logger.Error().Err(err).Str("auction_id", auctionID.String()).Msg("Failed to get highest bid")
		//return nil, err
	}

	// Update auction with winner information if there was a bid
	result := &shared.AuctionEndResult{
		AuctionID: auctionID,
		Status:    string(auction.Status),
	}

	if highestBid != nil {
		result.WinnerID = &highestBid.UserID
		result.FinalPrice = &highestBid.Amount

		client.logger.Info().
			Str("auction_id", auctionID.String()).
			Str("winner_id", highestBid.UserID.String()).
			Float64("final_price", highestBid.Amount).
			Msg("Auction ended with winner")
	} else {
		client.logger.Info().
			Str("auction_id", auctionID.String()).
			Msg("Auction ended with no bids")
	}

	// Update auction in database
	if err := client.auctionRepo.Update(ctx, auction); err != nil {
		client.logger.Error().Err(err).Str("auction_id", auctionID.String()).Msg("Failed to update auction in database")
		return nil, err
	}

	client.logger.Info().Str("auction_id", auctionID.String()).Msg("Auction ended successfully")
	return result, nil
}

// SetScheduler sets the auction scheduler
func (client *AuctionService) SetScheduler(scheduler *scheduler.AuctionScheduler) {
	client.scheduler = scheduler
}

// EndAuctionForScheduler implements scheduler.AuctionEndService interface
func (client *AuctionService) EndAuctionForScheduler(ctx context.Context, auctionID uuid.UUID) (*shared.AuctionEndResult, error) {
	return client.endAuctionWithResult(ctx, auctionID)
}
