package broadcaster

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"troffee-auction-service/internal/ports/outbound"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// RedisBroadcaster implements the broadcaster interface using Redis pub/sub
type RedisBroadcaster struct {
	client           *redis.Client
	subscribers      map[string]chan outbound.Event // clientID -> local channel
	pubsubs          map[string]*redis.PubSub       // clientID -> pubsub instance
	clientsToAuction map[string]map[string]bool     // clientID -> auctionID -> subscribed
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	logger           zerolog.Logger
}
type RedisBroadcasterParams struct {
	RedisClient *redis.Client
	Logger      zerolog.Logger
}

func NewBroadcaster(params RedisBroadcasterParams) *RedisBroadcaster {
	ctx, cancel := context.WithCancel(context.Background())

	broadcaster := &RedisBroadcaster{
		client:           params.RedisClient,
		subscribers:      make(map[string]chan outbound.Event),
		pubsubs:          make(map[string]*redis.PubSub),
		clientsToAuction: make(map[string]map[string]bool),
		ctx:              ctx,
		cancel:           cancel,
		logger:           params.Logger.With().Str("component", "redis_broadcaster").Logger(),
	}

	return broadcaster
}

// Subscribe subscribes a client to events for a specific auction
func (r *RedisBroadcaster) Subscribe(ctx context.Context, auctionID uuid.UUID, clientID string, eventChan chan outbound.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if client is already subscribed to this auction
	if r.clientsToAuction[clientID] != nil && r.clientsToAuction[clientID][auctionID.String()] {
		r.logger.Info().
			Str("client_id", clientID).
			Str("auction_id", auctionID.String()).
			Msg("Client already subscribed to auction")
		return nil
	}

	// Store the event channel if this is the first subscription
	if r.subscribers[clientID] == nil {
		r.subscribers[clientID] = eventChan
	}

	if r.clientsToAuction[clientID] == nil {
		r.clientsToAuction[clientID] = make(map[string]bool)
	}
	r.clientsToAuction[clientID][auctionID.String()] = true

	// Get or create pubsub connection for this client
	var pubsub *redis.PubSub
	if existingPubsub, exists := r.pubsubs[clientID]; exists {
		// Client already has a pubsub connection, subscribe to additional channel
		pubsub = existingPubsub
	} else {
		// Create new pubsub connection for this client
		pubsub = r.client.Subscribe(ctx)
		r.pubsubs[clientID] = pubsub

		// Start goroutine to listen for Redis messages and forward to local channel
		go r.listenForRedisMessages(pubsub, clientID, eventChan)
	}

	// Subscribe to the specific auction channel
	channelName := fmt.Sprintf("auction:%s", auctionID.String())
	if err := pubsub.Subscribe(ctx, channelName); err != nil {
		r.logger.Error().Err(err).Str("client_id", clientID).Str("auction_id", auctionID.String()).Msg("Failed to subscribe to Redis channel")
		return err
	}

	r.logger.Info().
		Str("client_id", clientID).
		Str("auction_id", auctionID.String()).
		Msg("Client subscribed to auction via Redis")
	return nil
}

// Unsubscribe unsubscribes a client from events for a specific auction
func (r *RedisBroadcaster) Unsubscribe(ctx context.Context, auctionID uuid.UUID, clientID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove auction tracking
	if clientAuctions, exists := r.clientsToAuction[clientID]; exists {
		delete(clientAuctions, auctionID.String())

		// If no more auctions, clean up the client entry
		if len(clientAuctions) == 0 {
			delete(r.clientsToAuction, clientID)

			// Close and remove local channel
			if eventChan, exists := r.subscribers[clientID]; exists {
				close(eventChan)
				delete(r.subscribers, clientID)
			}

			// Close Redis pubsub connection
			if pubsub, exists := r.pubsubs[clientID]; exists {
				if err := pubsub.Close(); err != nil {
					r.logger.Error().Err(err).Str("client_id", clientID).Msg("Error closing Redis pubsub for client")
				}
				delete(r.pubsubs, clientID)
			}
		} else {
			// Unsubscribe from the specific auction channel
			if pubsub, exists := r.pubsubs[clientID]; exists {
				channelName := fmt.Sprintf("auction:%s", auctionID.String())
				if err := pubsub.Unsubscribe(ctx, channelName); err != nil {
					r.logger.Error().Err(err).Str("client_id", clientID).Str("auction_id", auctionID.String()).Msg("Error unsubscribing from Redis channel")
				}
			}
		}
	}

	r.logger.Info().
		Str("client_id", clientID).
		Str("auction_id", auctionID.String()).
		Msg("Client unsubscribed from auction")
	return nil
}

// Publish publishes an event to all subscribers of an auction via Redis
func (r *RedisBroadcaster) Publish(ctx context.Context, auctionID uuid.UUID, event outbound.Event) error {
	channelName := fmt.Sprintf("auction:%s", auctionID.String())
	r.logger.Info().Str("channel_name", channelName).Msg("Publishing event to Redis")

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to marshal event")
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to Redis
	result := r.client.Publish(ctx, channelName, eventJSON)
	if err := result.Err(); err != nil {
		r.logger.Error().Err(err).Msg("Failed to publish to Redis")
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}

	subscriberCount := result.Val()
	r.logger.Info().
		Str("event_type", string(event.Type)).
		Str("auction_id", auctionID.String()).
		Int64("subscriber_count", subscriberCount).
		Msg("Published event to auction")

	return nil
}

func (r *RedisBroadcaster) GetSubscribers(ctx context.Context, auctionID uuid.UUID) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var subscribers []string
	for clientID := range r.subscribers {
		subscribers = append(subscribers, clientID)
	}

	return subscribers, nil
}

func (r *RedisBroadcaster) GetEventChannel(clientID string) <-chan outbound.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if eventChan, exists := r.subscribers[clientID]; exists {
		return eventChan
	}

	return nil
}

// listenForRedisMessages listens for Redis messages and forwards them to the local channel
func (r *RedisBroadcaster) listenForRedisMessages(pubsub *redis.PubSub, clientID string, localChan chan outbound.Event) {
	defer func() {
		if err := recover(); err != nil {
			r.logger.Error().Interface("panic", err).Str("client_id", clientID).Msg("Redis message listener panic for client")
		}
	}()

	ch := pubsub.Channel()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				r.logger.Info().Str("client_id", clientID).Msg("Redis channel closed for client")
				return
			}

			var event outbound.Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				r.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to unmarshal Redis message for client")
				continue
			}

			select {
			case localChan <- event:
			default:
				r.logger.Warn().Str("client_id", clientID).Msg("Local channel full for client, dropping event")
			}

		case <-r.ctx.Done():
			r.logger.Info().Str("client_id", clientID).Msg("Redis broadcaster context cancelled for client")
			return
		}
	}
}

func (r *RedisBroadcaster) Close() error {
	r.cancel()

	r.mu.Lock()
	defer r.mu.Unlock()

	for clientID, eventChan := range r.subscribers {
		close(eventChan)
		delete(r.subscribers, clientID)
	}

	// Close all pubsub connections
	for clientID, pubsub := range r.pubsubs {
		if err := pubsub.Close(); err != nil {
			r.logger.Error().Err(err).Str("client_id", clientID).Msg("Error closing Redis pubsub for client")
		}
		delete(r.pubsubs, clientID)
	}

	return r.client.Close()
}

func (r *RedisBroadcaster) IsSubscribed(ctx context.Context, auctionID uuid.UUID, clientID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clientAuctions, exists := r.clientsToAuction[clientID]
	if !exists {
		return false
	}

	return clientAuctions[auctionID.String()]
}
