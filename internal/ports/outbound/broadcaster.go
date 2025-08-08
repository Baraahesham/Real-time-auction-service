package outbound

import (
	"context"

	"github.com/google/uuid"
)

// EventType represents the type of event being broadcasted
type EventType string

const (
	EventTypeAuctionCreated EventType = "auction.created"
	EventTypeBidPlaced      EventType = "bid.placed"
	EventTypeAuctionEnded   EventType = "auction.ended"
	EventTypeError          EventType = "error"
)

// Event represents a broadcast event
type Event struct {
	Type      EventType              `json:"type"`
	AuctionID uuid.UUID              `json:"auction_id"`
	Data      map[string]interface{} `json:"data"`
	Timestamp int64                  `json:"timestamp"`
}

// Broadcaster defines the interface for broadcasting events
type Broadcaster interface {
	// Subscribe subscribes a client to events for a specific auction
	// When a client subscribes to multiple auctions, all events are delivered to the same channel
	Subscribe(ctx context.Context, auctionID uuid.UUID, clientID string, eventChan chan Event) error

	// Unsubscribe unsubscribes a client from events for a specific auction
	Unsubscribe(ctx context.Context, auctionID uuid.UUID, clientID string) error

	// Publish publishes an event to all subscribers of an auction
	Publish(ctx context.Context, auctionID uuid.UUID, event Event) error

	// GetSubscribers returns the list of client IDs subscribed to an auction
	GetSubscribers(ctx context.Context, auctionID uuid.UUID) ([]string, error)

	// IsSubscribed checks if a client is subscribed to an auction
	IsSubscribed(ctx context.Context, auctionID uuid.UUID, clientID string) bool
}
