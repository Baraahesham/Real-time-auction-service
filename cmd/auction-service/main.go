package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"troffee-auction-service/internal/adapters/broadcaster"
	"troffee-auction-service/internal/adapters/db"
	"troffee-auction-service/internal/adapters/redis"
	"troffee-auction-service/internal/adapters/scheduler"
	"troffee-auction-service/internal/adapters/ws"
	"troffee-auction-service/internal/app"
	"troffee-auction-service/internal/config"
)

func main() {

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	initLogging(cfg)

	log.Info().Msg("Starting Trofee Auction Service...")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database connection
	dbConn, err := db.NewConnection(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer dbConn.Close()

	log.Info().Msg("Database connection established")

	// Create repositories
	repoFactory := db.NewRepositoryFactory(dbConn)
	auctionRepo := repoFactory.GetAuctionRepository()
	bidRepo := repoFactory.GetBidRepository()
	itemRepo := repoFactory.GetItemRepository()
	userRepo := repoFactory.GetUserRepository()

	log.Info().Msg("Database repositories initialized")

	// Create Redis client
	redisClient := redis.NewClient(cfg)
	if err := redis.PingRedis(redisClient); err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	log.Info().Msg("Redis connection established")

	// Create Redis broadcaster
	redisBroadcaster := broadcaster.NewBroadcaster(broadcaster.RedisBroadcasterParams{
		RedisClient: redisClient,
		Logger:      log.Logger,
	})
	log.Info().Msg("Redis broadcaster initialized")

	// Create business services
	auctionService := app.NewAuctionService(app.AuctionServiceParams{
		AuctionRepo: auctionRepo,
		ItemRepo:    itemRepo,
		UserRepo:    userRepo,
		BidRepo:     bidRepo,
		Logger:      log.Logger,
	})
	bidService := app.NewBidService(app.BidServiceParams{
		BidRepo:     bidRepo,
		AuctionRepo: auctionRepo,
		UserRepo:    userRepo,
		Broadcaster: redisBroadcaster,
		Logger:      log.Logger,
	})

	log.Info().Msg("Business services initialized")

	// Create auction scheduler
	auctionScheduler := scheduler.NewAuctionScheduler(
		scheduler.AuctionSchedulerParams{
			RedisClient:    redisClient,
			AuctionService: auctionService,
			Broadcaster:    redisBroadcaster,
			Logger:         log.Logger,
		},
	)

	// Start auction scheduler
	auctionScheduler.Start()
	log.Info().Msg("Auction scheduler started")

	// Update auction service with scheduler
	auctionService.SetScheduler(auctionScheduler)

	wsServer := ws.NewServer(ws.ServerParams{
		Config:         cfg,
		AuctionService: auctionService,
		BidService:     bidService,
		Broadcaster:    redisBroadcaster,
		Logger:         log.Logger,
	})

	log.Info().Msg("WebSocket server initialized")

	// Start WebSocket server
	go func() {
		log.Info().Str("port", cfg.Server.Port).Msg("Starting WebSocket server")
		if err := wsServer.Start(); err != nil {
			log.Error().Err(err).Msg("Failed to start WebSocket server")
			cancel()
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	case <-ctx.Done():
		log.Info().Msg("Context cancelled")
	}

	// Graceful shutdown
	log.Info().Msg("Starting graceful shutdown...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop auction scheduler
	auctionScheduler.Stop()
	log.Info().Msg("Auction scheduler stopped")

	// Stop WebSocket server
	if err := wsServer.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error stopping WebSocket server")
	}

	log.Info().Msg("Graceful shutdown completed")
}

func initLogging(cfg *config.Config) {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Set log format
	if cfg.Logging.Format == "json" {
		// JSON format (default)
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	} else {
		// Console format for development
		output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
		log.Logger = zerolog.New(output).With().Timestamp().Logger()
	}

	// Set global logger
	zerolog.DefaultContextLogger = &log.Logger
}
