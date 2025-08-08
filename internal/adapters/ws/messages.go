package ws

import (
	"encoding/json"
	"fmt"
	"time"

	"troffee-auction-service/internal/domain/shared"

	"github.com/google/uuid"
)

type MessageType string

const (
	// Client to Server message types
	MessageTypeSubscribe     MessageType = "subscribe"
	MessageTypeUnsubscribe   MessageType = "unsubscribe"
	MessageTypePlaceBid      MessageType = "place_bid"
	MessageTypeCreateAuction MessageType = "create_auction"
	MessageTypeGetAuction    MessageType = "get_auction"
	MessageTypeListAuctions  MessageType = "list_auctions"
	MessageTypePing          MessageType = "ping"

	// Server to Client message types
	MessageTypeBidPlaced      MessageType = "bid_placed"
	MessageTypeAuctionEnded   MessageType = "auction_ended"
	MessageTypeAuctionUpdate  MessageType = "auction_update"
	MessageTypeAuctionCreated MessageType = "auction_created"
	MessageTypeError          MessageType = "error"
	MessageTypePong           MessageType = "pong"
)

type ClientMessage struct {
	Type      MessageType            `json:"type"`
	AuctionID *uuid.UUID             `json:"auction_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

// ServerMessage represents a message sent from server to client
type ServerMessage struct {
	Type      MessageType            `json:"type"`
	AuctionID *uuid.UUID             `json:"auction_id,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     *string                `json:"error,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

// BidData represents bid information in messages
type BidData struct {
	BidID     uuid.UUID `json:"bid_id"`
	UserID    uuid.UUID `json:"user_id"`
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
}

type AuctionData struct {
	AuctionID    uuid.UUID `json:"auction_id"`
	CurrentPrice float64   `json:"current_price"`
	Status       string    `json:"status"`
	EndTime      time.Time `json:"end_time"`
}

func NewServerMessage(msgType MessageType) *ServerMessage {
	return &ServerMessage{
		Type:      msgType,
		Data:      make(map[string]interface{}),
		Timestamp: time.Now().Unix(),
	}
}

func NewErrorMessage(err string, auctionID *uuid.UUID) *ServerMessage {
	return &ServerMessage{
		Type:      MessageTypeError,
		AuctionID: auctionID,
		Error:     &err,
		Timestamp: time.Now().Unix(),
	}
}

// NewAuctionEndedMessage creates an auction ended message
func NewAuctionEndedMessage(auctionID uuid.UUID, winnerID *uuid.UUID, finalPrice float64) *ServerMessage {
	msg := NewServerMessage(MessageTypeAuctionEnded)
	msg.AuctionID = &auctionID
	msg.Data["final_price"] = finalPrice
	if winnerID != nil {
		msg.Data["winner_id"] = winnerID
	}
	return msg
}
func (m *ClientMessage) validateAuctionID() error {
	if m.AuctionID == nil || *m.AuctionID == uuid.Nil {
		return shared.ErrAuctionIDRequired
	}
	return nil
}

// ParseClientMessage parses a JSON message from client
func ParseClientMessage(data []byte) (*ClientMessage, error) {
	var msg ClientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse client message: %w", err)
	}

	// Validate required fields
	if msg.Type == "" {
		return nil, shared.ErrMessageTypeRequired
	}

	return &msg, nil
}

// Validate validates a client message
func (m *ClientMessage) Validate() error {
	switch m.Type {
	case MessageTypeSubscribe, MessageTypeUnsubscribe:
		if err := m.validateAuctionID(); err != nil {
			return err
		}
	case MessageTypePlaceBid:
		if err := m.validateAuctionID(); err != nil {
			return err
		}
		amount, ok := m.Data["amount"].(float64)
		if !ok || amount <= 0 {
			return shared.ErrInvalidAmount
		}
	case MessageTypeCreateAuction:
		if m.Data["item_id"] == nil {
			return shared.ErrItemIDRequired
		}
		if m.Data["start_time"] == nil {
			return shared.ErrStartTimeRequired
		}
		if m.Data["end_time"] == nil {
			return shared.ErrEndTimeRequired
		}
		if m.Data["starting_price"] == nil {
			return shared.ErrStartingPriceRequired
		}
	case MessageTypeGetAuction:
		if err := m.validateAuctionID(); err != nil {
			return err
		}
	case MessageTypeListAuctions:

	case MessageTypePing:

	default:
		return shared.ErrUnknownMessageType
	}

	return nil
}
