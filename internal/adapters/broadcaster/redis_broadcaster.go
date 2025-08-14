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
func (redisClient *RedisBroadcaster) Subscribe(ctx context.Context, auctionID uuid.UUID, clientID string, eventChan chan outbound.Event) error {
	redisClient.mu.Lock()
	defer redisClient.mu.Unlock()

	// Check if client is already subscribed to this auction
	if redisClient.clientsToAuction[clientID] != nil && redisClient.clientsToAuction[clientID][auctionID.String()] {
		redisClient.logger.Info().
			Str("client_id", clientID).
			Str("auction_id", auctionID.String()).
			Msg("Client already subscribed to auction")
		return nil
	}

	// Store the event channel if this is the first subscription
	if redisClient.subscribers[clientID] == nil {
		redisClient.subscribers[clientID] = eventChan
	}

	if redisClient.clientsToAuction[clientID] == nil {
		redisClient.clientsToAuction[clientID] = make(map[string]bool)
	}
	redisClient.clientsToAuction[clientID][auctionID.String()] = true

	// Get or create pubsub connection for this client
	var pubsub *redis.PubSub
	if existingPubsub, exists := redisClient.pubsubs[clientID]; exists {
		// Client already has a pubsub connection, subscribe to additional channel
		pubsub = existingPubsub
	} else {
		// Create new pubsub connection for this client
		pubsub = redisClient.client.Subscribe(ctx)
		redisClient.pubsubs[clientID] = pubsub

		// Start goroutine to listen for Redis messages and forward to local channel
		go redisClient.listenForRedisMessages(pubsub, clientID, eventChan)
	}

	// Subscribe to the specific auction channel
	channelName := fmt.Sprintf("auction:%s", auctionID.String())
	if err := pubsub.Subscribe(ctx, channelName); err != nil {
		redisClient.logger.Error().Err(err).Str("client_id", clientID).Str("auction_id", auctionID.String()).Msg("Failed to subscribe to Redis channel")
		return err
	}

	redisClient.logger.Info().
		Str("client_id", clientID).
		Str("auction_id", auctionID.String()).
		Msg("Client subscribed to auction via Redis")
	return nil
}

// Unsubscribe unsubscribes a client from events for a specific auction
func (redisClient *RedisBroadcaster) Unsubscribe(ctx context.Context, auctionID uuid.UUID, clientID string) error {
	redisClient.mu.Lock()
	defer redisClient.mu.Unlock()

	if clientAuctions, exists := redisClient.clientsToAuction[clientID]; exists {
		delete(clientAuctions, auctionID.String())

		// If no more auctions, clean up the client entry
		if len(clientAuctions) == 0 {
			delete(redisClient.clientsToAuction, clientID)

			// Close and remove local channel
			if eventChan, exists := redisClient.subscribers[clientID]; exists {
				close(eventChan)
				delete(redisClient.subscribers, clientID)
			}

			// Close Redis pubsub connection
			if pubsub, exists := redisClient.pubsubs[clientID]; exists {
				if err := pubsub.Close(); err != nil {
					redisClient.logger.Error().Err(err).Str("client_id", clientID).Msg("Error closing Redis pubsub for client")
				}
				delete(redisClient.pubsubs, clientID)
			}
		} else {
			// Unsubscribe from the specific auction channel
			if pubsub, exists := redisClient.pubsubs[clientID]; exists {
				channelName := fmt.Sprintf("auction:%s", auctionID.String())
				if err := pubsub.Unsubscribe(ctx, channelName); err != nil {
					redisClient.logger.Error().Err(err).Str("client_id", clientID).Str("auction_id", auctionID.String()).Msg("Error unsubscribing from Redis channel")
				}
			}
		}
	}

	redisClient.logger.Info().
		Str("client_id", clientID).
		Str("auction_id", auctionID.String()).
		Msg("Client unsubscribed from auction")
	return nil
}

// Publish publishes an event to all subscribers of an auction via Redis
func (redisClient *RedisBroadcaster) Publish(ctx context.Context, auctionID uuid.UUID, event outbound.Event) error {
	channelName := fmt.Sprintf("auction:%s", auctionID.String())
	redisClient.logger.Info().Str("channel_name", channelName).Msg("Publishing event to Redis")

	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		redisClient.logger.Error().Err(err).Msg("Failed to marshal event")
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish to Redis
	result := redisClient.client.Publish(ctx, channelName, eventJSON)
	if err := result.Err(); err != nil {
		redisClient.logger.Error().Err(err).Msg("Failed to publish to Redis")
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}

	subscriberCount := result.Val()
	redisClient.logger.Info().
		Str("event_type", string(event.Type)).
		Str("auction_id", auctionID.String()).
		Int64("subscriber_count", subscriberCount).
		Msg("Published event to auction")

	return nil
}

func (redisClient *RedisBroadcaster) GetSubscribers(ctx context.Context, auctionID uuid.UUID) ([]string, error) {
	redisClient.mu.RLock()
	defer redisClient.mu.RUnlock()

	var subscribers []string
	for clientID := range redisClient.subscribers {
		subscribers = append(subscribers, clientID)
	}

	return subscribers, nil
}

func (redisClient *RedisBroadcaster) GetEventChannel(clientID string) <-chan outbound.Event {
	redisClient.mu.RLock()
	defer redisClient.mu.RUnlock()

	if eventChan, exists := redisClient.subscribers[clientID]; exists {
		return eventChan
	}

	return nil
}

// listenForRedisMessages listens for Redis messages and forwards them to the local channel
func (redisClient *RedisBroadcaster) listenForRedisMessages(pubsub *redis.PubSub, clientID string, localChan chan outbound.Event) {
	defer func() {
		if err := recover(); err != nil {
			redisClient.logger.Error().Interface("panic", err).Str("client_id", clientID).Msg("Redis message listener panic for client")
		}
	}()

	ch := pubsub.Channel()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				redisClient.logger.Info().Str("client_id", clientID).Msg("Redis channel closed for client")
				return
			}

			var event outbound.Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				redisClient.logger.Error().Err(err).Str("client_id", clientID).Msg("Failed to unmarshal Redis message for client")
				continue
			}

			select {
			case localChan <- event:
			default:
				redisClient.logger.Warn().Str("client_id", clientID).Msg("Local channel full for client, dropping event")
			}

		case <-redisClient.ctx.Done():
			redisClient.logger.Info().Str("client_id", clientID).Msg("Redis broadcaster context cancelled for client")
			return
		}
	}
}

func (redisClient *RedisBroadcaster) Close() error {
	redisClient.cancel()

	redisClient.mu.Lock()
	defer redisClient.mu.Unlock()

	for clientID, eventChan := range redisClient.subscribers {
		close(eventChan)
		delete(redisClient.subscribers, clientID)
	}

	// Close all pubsub connections
	for clientID, pubsub := range redisClient.pubsubs {
		if err := pubsub.Close(); err != nil {
			redisClient.logger.Error().Err(err).Str("client_id", clientID).Msg("Error closing Redis pubsub for client")
		}
		delete(redisClient.pubsubs, clientID)
	}

	return redisClient.client.Close()
}

func (redisClient *RedisBroadcaster) IsSubscribed(ctx context.Context, auctionID uuid.UUID, clientID string) bool {
	redisClient.mu.RLock()
	defer redisClient.mu.RUnlock()

	clientAuctions, exists := redisClient.clientsToAuction[clientID]
	if !exists {
		return false
	}

	return clientAuctions[auctionID.String()]
}
