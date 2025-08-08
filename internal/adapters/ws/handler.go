package ws

import (
	"context"
	"net/http"
	"sync"
	"time"

	"troffee-auction-service/internal/domain/auction"
	"troffee-auction-service/internal/domain/shared"
	"troffee-auction-service/internal/ports/inbound"
	"troffee-auction-service/internal/ports/outbound"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// WsHandler manages WebSocket connections and message routing
type WsHandler struct {
	clients        map[string]*WsClient // clientID -> Client
	clientsMu      sync.RWMutex
	eventChannels  map[string]chan outbound.Event // clientID -> local event channel
	channelsMu     sync.RWMutex
	upgrader       websocket.Upgrader
	auctionService inbound.AuctionService
	bidService     inbound.BidService
	broadcaster    outbound.Broadcaster
	logger         zerolog.Logger
}
type WsHandlerParams struct {
	Upgrader       websocket.Upgrader
	AuctionService inbound.AuctionService
	BidService     inbound.BidService
	Broadcaster    outbound.Broadcaster
	Logger         zerolog.Logger
}

// NewHandler creates a new WebSocket handler
func NewHandler(params WsHandlerParams) *WsHandler {
	return &WsHandler{
		clients:        make(map[string]*WsClient),
		eventChannels:  make(map[string]chan outbound.Event),
		upgrader:       params.Upgrader,
		auctionService: params.AuctionService,
		bidService:     params.BidService,
		broadcaster:    params.Broadcaster,
		logger:         params.Logger.With().Str("component", "ws_handler").Logger(),
	}
}

// HandleWebSocket handles WebSocket connection upgrades
func (handler *WsHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "invalid user_id format", http.StatusBadRequest)
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := handler.upgrader.Upgrade(w, r, nil)
	if err != nil {
		handler.logger.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}

	// Create new client
	client := NewClient(WsClientParams{
		UserID:  userID,
		Conn:    conn,
		Handler: handler,
	})

	// Register client
	handler.registerClient(client)

	// Create local event channel for this client
	handler.createEventChannel(client.id)

	// Start client message handling
	client.Start()

	// Start listening for broadcast events for this client
	go handler.listenForClientEvents(client)

	// Wait for client to disconnect
	go func() {
		<-client.ctx.Done()
		handler.unregisterClient(client)
	}()

	handler.logger.Info().Str("client_id", client.id).Str("user_id", client.userID.String()).Msg("WebSocket client connected")
}

// createEventChannel creates a local event channel for a client
func (handler *WsHandler) createEventChannel(clientID string) chan outbound.Event {
	handler.channelsMu.Lock()
	defer handler.channelsMu.Unlock()

	if eventChan, exists := handler.eventChannels[clientID]; exists {
		return eventChan
	}

	eventChan := make(chan outbound.Event, 100)
	handler.eventChannels[clientID] = eventChan

	handler.logger.Debug().Str("client_id", clientID).Msg("Created local event channel for client")
	return eventChan
}

func (handler *WsHandler) getEventChannel(clientID string) chan outbound.Event {
	handler.channelsMu.RLock()
	defer handler.channelsMu.RUnlock()

	return handler.eventChannels[clientID]
}

func (handler *WsHandler) removeEventChannel(clientID string) {
	handler.channelsMu.Lock()
	defer handler.channelsMu.Unlock()

	if eventChan, exists := handler.eventChannels[clientID]; exists {
		close(eventChan)
		delete(handler.eventChannels, clientID)
		handler.logger.Debug().Str("client_id", clientID).Msg("Removed local event channel for client")
	}
}

func (handler *WsHandler) registerClient(client *WsClient) {
	handler.clientsMu.Lock()
	defer handler.clientsMu.Unlock()
	handler.clients[client.id] = client
	handler.logger.Debug().Str("client_id", client.id).Int("total_clients", len(handler.clients)).Msg("Client registered")
}

func (handler *WsHandler) unregisterClient(client *WsClient) {
	handler.clientsMu.Lock()
	defer handler.clientsMu.Unlock()

	// Remove client from registry
	delete(handler.clients, client.id)

	// Note: Redis broadcaster handles subscription cleanup automatically
	// No need to manually unsubscribe - Redis will clean up when client disconnects

	// Stop the client
	client.Stop()

	// Remove local event channel
	handler.removeEventChannel(client.id)

	handler.logger.Info().Str("client_id", client.id).Str("user_id", client.userID.String()).Int("total_clients", len(handler.clients)).Msg("WebSocket client disconnected")
}

// listenForClientEvents listens for redis events
func (handler *WsHandler) listenForClientEvents(client *WsClient) {
	handler.logger.Debug().Str("client_id", client.id).Msg("Starting event listener for client")

	// Get the local event channel for this client
	eventChan := handler.getEventChannel(client.id)
	if eventChan == nil {
		handler.logger.Error().Str("client_id", client.id).Msg("No event channel found for client - this should not happen")
		return
	}

	handler.logger.Info().Str("client_id", client.id).Msg("Event listener started for client")

	// Listen for events and forward to WebSocket
	for {
		select {
		case event := <-eventChan:
			handler.logger.Debug().Str("client_id", client.id).Msg("Received event for client")
			wsMessage := handler.convertEventToMessage(event)

			if err := client.Send(wsMessage); err != nil {
				handler.logger.Error().
					Err(err).Str("client_id", client.id).Msg("Failed to send event to WebSocket client")
			} else {
				handler.logger.Info().Str("client_id", client.id).Str("event_type", string(event.Type)).
					Msg("Successfully sent event to WebSocket client")
			}

		case <-client.ctx.Done():
			handler.logger.Debug().Str("client_id", client.id).Msg("Client disconnected, stopping event listener")
			return
		}
	}
}

func (handler *WsHandler) HandleClientMessage(client *WsClient, msg *ClientMessage) error {
	switch msg.Type {
	case MessageTypeSubscribe:
		return handler.handleSubscribe(client, msg)

	case MessageTypeUnsubscribe:
		return handler.handleUnsubscribe(client, msg)

	case MessageTypePlaceBid:
		return handler.handlePlaceBid(client, msg)

	case MessageTypeCreateAuction:
		return handler.handleCreateAuction(client, msg)

	case MessageTypeGetAuction:
		return handler.handleGetAuction(client, msg)

	case MessageTypeListAuctions:
		return handler.handleListAuctions(client, msg)

	default:
		handler.logger.Warn().Str("client_id", client.id).Str("message_type", string(msg.Type)).Msg("Unknown message type from client")
		return shared.ErrUnknownMessageType
	}
}

func (handler *WsHandler) convertEventToMessage(event outbound.Event) *ServerMessage {
	switch event.Type {
	case outbound.EventTypeBidPlaced:
		return &ServerMessage{
			Type:      MessageTypeBidPlaced,
			AuctionID: &event.AuctionID,
			Data:      event.Data,
			Timestamp: event.Timestamp,
		}
	case outbound.EventTypeAuctionEnded:
		return &ServerMessage{
			Type:      MessageTypeAuctionEnded,
			AuctionID: &event.AuctionID,
			Data:      event.Data,
			Timestamp: event.Timestamp,
		}
	default:
		return &ServerMessage{
			Type:      MessageTypeAuctionUpdate,
			AuctionID: &event.AuctionID,
			Data:      event.Data,
			Timestamp: event.Timestamp,
		}
	}
}

// GetConnectedClients returns the number of connected clients
func (handler *WsHandler) GetConnectedClients() int {
	handler.clientsMu.RLock()
	defer handler.clientsMu.RUnlock()
	return len(handler.clients)
}

func (handler *WsHandler) handleSubscribe(client *WsClient, msg *ClientMessage) error {
	if msg.AuctionID == nil {
		return shared.ErrAuctionIDRequired
	}

	ctx := context.Background()

	eventChan := handler.getEventChannel(client.id)
	if eventChan == nil {
		handler.logger.Error().Str("client_id", client.id).Msg("No event channel found for client")
		return shared.ErrClientEventChannelNotFound
	}

	// Subscribe to broadcaster with the local event channel
	if err := handler.broadcaster.Subscribe(ctx, *msg.AuctionID, client.id, eventChan); err != nil {
		handler.logger.Error().Err(err).Str("client_id", client.id).Str("auction_id", msg.AuctionID.String()).Msg("Failed to subscribe to auction")
		return err
	}

	response := NewServerMessage(MessageTypeAuctionUpdate)
	response.AuctionID = msg.AuctionID
	response.Data["status"] = "subscribed"

	handler.logger.Info().Str("client_id", client.id).Str("auction_id", msg.AuctionID.String()).Msg("Client subscribed to auction")
	return client.Send(response)
}

// handleUnsubscribe handles unsubscription from auction events
func (handler *WsHandler) handleUnsubscribe(client *WsClient, msg *ClientMessage) error {
	if msg.AuctionID == nil {
		return shared.ErrAuctionIDRequired
	}

	ctx := context.Background()

	// Unsubscribe from broadcaster
	if err := handler.broadcaster.Unsubscribe(ctx, *msg.AuctionID, client.id); err != nil {
		return err
	}

	// Send confirmation
	response := NewServerMessage(MessageTypeAuctionUpdate)
	response.AuctionID = msg.AuctionID
	response.Data["status"] = "unsubscribed"

	handler.logger.Info().Str("client_id", client.id).Str("auction_id", msg.AuctionID.String()).Msg("Client unsubscribed from auction")
	return client.Send(response)
}

// handlePlaceBid handles bid placement
func (handler *WsHandler) handlePlaceBid(client *WsClient, msg *ClientMessage) error {
	if msg.AuctionID == nil {
		return shared.ErrAuctionIDRequired
	}

	amount, ok := msg.Data["amount"].(float64)
	if !ok {
		return shared.ErrInvalidAmount
	}

	ctx := context.Background()

	// Create bid request
	bidRequest := inbound.PlaceBidRequest{
		AuctionID: *msg.AuctionID,
		UserID:    client.userID,
		ClientID:  client.id,
		Amount:    amount,
	}

	// Place bid through application service
	bid, err := handler.bidService.PlaceBid(ctx, bidRequest)
	if err != nil {
		// Send error message back to client
		errorMsg := NewErrorMessage(err.Error(), msg.AuctionID)
		return client.Send(errorMsg)
	}

	handler.logger.Info().Str("bid_id", bid.ID.String()).Str("auction_id", msg.AuctionID.String()).Str("user_id", client.userID.String()).Float64("amount", amount).Msg("Bid placed successfully")

	return nil
}

// handleCreateAuction handles auction creation
func (handler *WsHandler) handleCreateAuction(client *WsClient, msg *ClientMessage) error {
	ctx := context.Background()

	// Extract auction data
	itemIDStr, ok := msg.Data["item_id"].(string)
	if !ok {
		return shared.ErrItemIDRequired
	}

	itemID, err := uuid.Parse(itemIDStr)
	if err != nil {
		return shared.ErrInvalidItemIDFormat
	}

	startTimeStr, ok := msg.Data["start_time"].(string)
	if !ok {
		return shared.ErrStartTimeRequired
	}

	endTimeStr, ok := msg.Data["end_time"].(string)
	if !ok {
		return shared.ErrEndTimeRequired
	}

	startingPrice, ok := msg.Data["starting_price"].(float64)
	if !ok {
		return shared.ErrStartingPriceRequired
	}

	// Create auction request
	auctionRequest := inbound.CreateAuctionRequest{
		ItemID:        itemID,
		CreatorID:     client.userID,
		StartTime:     startTimeStr,
		EndTime:       endTimeStr,
		StartingPrice: startingPrice,
	}

	// Create auction through application service
	auction, err := handler.auctionService.CreateAuction(ctx, auctionRequest)
	if err != nil {
		errorMsg := NewErrorMessage(err.Error(), nil)
		return client.Send(errorMsg)
	}

	// Send success response
	response := handler.createAuctionResponse(auction, MessageTypeAuctionCreated, nil)

	handler.logger.Info().Str("auction_id", auction.ID.String()).Str("user_id", client.userID.String()).Msg("Auction created successfully")
	return client.Send(response)
}

// handleGetAuction handles getting auction details
func (handler *WsHandler) handleGetAuction(client *WsClient, msg *ClientMessage) error {
	if msg.AuctionID == nil {
		return shared.ErrAuctionIDRequired
	}

	ctx := context.Background()

	auction, err := handler.auctionService.GetAuction(ctx, *msg.AuctionID)
	if err != nil {
		errorMsg := NewErrorMessage(err.Error(), msg.AuctionID)
		return client.Send(errorMsg)
	}

	response := handler.createAuctionResponse(auction, MessageTypeAuctionUpdate, msg.AuctionID)

	return client.Send(response)
}

// handleListAuctions handles listing auctions
func (handler *WsHandler) handleListAuctions(client *WsClient, msg *ClientMessage) error {
	ctx := context.Background()

	limit := 10
	if limitVal, ok := msg.Data["limit"].(float64); ok {
		limit = int(limitVal)
	}

	offset := 0
	if offsetVal, ok := msg.Data["offset"].(float64); ok {
		offset = int(offsetVal)
	}

	auctionRequest := inbound.ListAuctionsRequest{
		Page:     offset/limit + 1, // Convert offset to page
		PageSize: limit,
		Status:   nil,
	}

	// Get auctions through application service
	auctions, err := handler.auctionService.ListAuctions(ctx, auctionRequest)
	if err != nil {
		errorMsg := NewErrorMessage(err.Error(), nil)
		return client.Send(errorMsg)
	}

	// Send auctions data
	response := NewServerMessage(MessageTypeAuctionUpdate)
	response.Data["auctions"] = auctions
	response.Data["count"] = len(auctions)

	return client.Send(response)
}

func (handler *WsHandler) createAuctionResponse(auction *auction.Auction, msgType MessageType, auctionID *uuid.UUID) *ServerMessage {
	response := NewServerMessage(msgType)
	if auctionID != nil {
		response.AuctionID = auctionID
	}

	response.Data["auction_id"] = auction.ID
	response.Data["item_id"] = auction.ItemID
	response.Data["creator_id"] = auction.CreatorID
	response.Data["start_time"] = auction.StartTime.Format(time.RFC3339)
	response.Data["end_time"] = auction.EndTime.Format(time.RFC3339)
	response.Data["starting_price"] = auction.StartingPrice
	response.Data["current_price"] = auction.CurrentPrice
	response.Data["status"] = auction.Status

	return response
}
