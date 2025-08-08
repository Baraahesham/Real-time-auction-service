package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"troffee-auction-service/internal/domain/shared"
	"troffee-auction-service/internal/ports/outbound"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type AuctionEndService interface {
	EndAuctionForScheduler(ctx context.Context, auctionID uuid.UUID) (*shared.AuctionEndResult, error)
}

type AuctionScheduler struct {
	redis          *redis.Client
	auctionService AuctionEndService
	broadcaster    outbound.Broadcaster
	logger         zerolog.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}
type AuctionSchedulerParams struct {
	RedisClient    *redis.Client
	AuctionService AuctionEndService
	Broadcaster    outbound.Broadcaster
	Logger         zerolog.Logger
}

func NewAuctionScheduler(params AuctionSchedulerParams) *AuctionScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &AuctionScheduler{
		redis:          params.RedisClient,
		auctionService: params.AuctionService,
		broadcaster:    params.Broadcaster,
		logger:         params.Logger.With().Str("component", "auction_scheduler").Logger(),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// ScheduleAuction adds an auction to the expiration schedule
func (s *AuctionScheduler) ScheduleAuction(auctionID uuid.UUID, endTime time.Time) error {
	score := float64(endTime.Unix())

	err := s.redis.ZAdd(s.ctx, "auction:expirations", redis.Z{
		Score:  score,
		Member: auctionID.String(),
	}).Err()

	if err != nil {
		s.logger.Error().Err(err).Str("auction_id", auctionID.String()).Msg("Failed to schedule auction")
		return fmt.Errorf("failed to schedule auction: %w", err)
	}

	s.logger.Info().
		Str("auction_id", auctionID.String()).
		Time("end_time", endTime).
		Msg("Auction scheduled for expiration")

	return nil
}

// Start begins the scheduler loop
func (s *AuctionScheduler) Start() {
	s.logger.Info().Msg("Starting auction scheduler")

	// Start the main scheduler loop
	s.wg.Add(1)
	go s.schedulerLoop()
}

// Stop gracefully stops the scheduler
func (s *AuctionScheduler) Stop() {
	s.logger.Info().Msg("Stopping auction scheduler")
	s.cancel()
	s.wg.Wait()
}

// schedulerLoop runs the main scheduling loop
func (s *AuctionScheduler) schedulerLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkExpiredAuctions()
		case <-s.ctx.Done():
			s.logger.Info().Msg("Scheduler loop stopped")
			return
		}
	}
}

// checkExpiredAuctions finds and processes expired auctions
func (s *AuctionScheduler) checkExpiredAuctions() {
	now := time.Now().Unix()

	// Get expired auctions using ZRANGEBYSCORE
	expiredAuctions, err := s.redis.ZRangeByScore(s.ctx, "auction:expirations", &redis.ZRangeBy{
		Min:   "0",
		Max:   strconv.FormatInt(now, 10),
		Count: 10, // Process max 10 at a time
	}).Result()

	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get expired auctions")
		return
	}

	if len(expiredAuctions) > 0 {
		s.logger.Debug().Int("count", len(expiredAuctions)).Msg("Found expired auctions")
	}

	for _, auctionIDStr := range expiredAuctions {
		auctionID, err := uuid.Parse(auctionIDStr)
		if err != nil {
			s.logger.Error().Err(err).Str("auction_id", auctionIDStr).Msg("Invalid auction ID")
			continue
		}

		// Process auction end
		go s.endAuction(auctionID)
	}
}

// endAuction processes the end of an auction
func (s *AuctionScheduler) endAuction(auctionID uuid.UUID) {
	s.logger.Info().Str("auction_id", auctionID.String()).Msg("Processing auction end")

	// End the auction
	result, err := s.auctionService.EndAuctionForScheduler(s.ctx, auctionID)
	defer s.redis.ZRem(s.ctx, "auction:expirations", auctionID.String())

	if err != nil {
		s.logger.Error().Err(err).Str("auction_id", auctionID.String()).Msg("Failed to end auction")
		return
	}

	// Broadcast result
	eventData := map[string]interface{}{
		"auction_id": auctionID.String(),
		"status":     result.Status,
	}
	if result.WinnerID != nil {
		eventData["winner_id"] = result.WinnerID.String()
	}
	if result.FinalPrice != nil {
		eventData["final_price"] = *result.FinalPrice
	}

	event := outbound.Event{
		Type:      outbound.EventTypeAuctionEnded,
		AuctionID: auctionID,
		Data:      eventData,
		Timestamp: time.Now().Unix(),
	}

	// Broadcast to all subscribers
	if err := s.broadcaster.Publish(s.ctx, auctionID, event); err != nil {
		s.logger.Error().Err(err).Str("auction_id", auctionID.String()).Msg("Failed to broadcast auction end event")
	}

	logger := s.logger.Info().Str("auction_id", auctionID.String())

	if result.WinnerID != nil {
		logger = logger.Str("winner_id", result.WinnerID.String())
	}
	if result.FinalPrice != nil {
		logger = logger.Float64("final_price", *result.FinalPrice)
	}

	logger.Msg("Auction ended successfully")
}
