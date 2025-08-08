package ws

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"troffee-auction-service/internal/config"
	"troffee-auction-service/internal/ports/inbound"
	"troffee-auction-service/internal/ports/outbound"

	"github.com/rs/zerolog"
)

type Server struct {
	handler    *WsHandler
	httpServer *http.Server
	config     *config.Config
	logger     zerolog.Logger
}

type ServerParams struct {
	Config         *config.Config
	AuctionService inbound.AuctionService
	BidService     inbound.BidService
	Broadcaster    outbound.Broadcaster
	Logger         zerolog.Logger
}

func NewServer(params ServerParams) *Server {
	handler := NewHandler(WsHandlerParams{
		AuctionService: params.AuctionService,
		BidService:     params.BidService,
		Broadcaster:    params.Broadcaster,
		Logger:         params.Logger,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handler.HandleWebSocket)
	mux.HandleFunc("/health", handleHealth)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", params.Config.Server.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Minute,
	}

	return &Server{
		handler:    handler,
		httpServer: httpServer,
		config:     params.Config,
		logger:     params.Logger,
	}
}

// Start starts the WebSocket server
func (s *Server) Start() error {
	s.logger.Info().Str("port", s.config.Server.Port).Msg("Starting WebSocket server")

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start WebSocket server: %w", err)
	}

	return nil
}

// Stop gracefully stops the WebSocket server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info().Msg("Stopping WebSocket server...")

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown WebSocket server: %w", err)
	}

	s.logger.Info().Msg("WebSocket server stopped")
	return nil
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok", "service": "auction-websocket"}`))
}
