package auction

import (
	"time"

	"github.com/google/uuid"
)

// Status represents the current status of an auction
type Status string

const (
	StatusPending   Status = "pending"
	StatusActive    Status = "active"
	StatusEnded     Status = "ended"
	StatusCancelled Status = "cancelled"
)

// Auction represents an auction for an item
type Auction struct {
	ID            uuid.UUID `json:"id"`
	ItemID        uuid.UUID `json:"item_id"`
	CreatorID     uuid.UUID `json:"creator_id"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	StartingPrice float64   `json:"starting_price"`
	CurrentPrice  float64   `json:"current_price"`
	Status        Status    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// IsActive returns true if the auction is currently active
func (a *Auction) IsActive() bool {
	return a.Status == StatusActive
}

// IsEnded returns true if the auction has ended
func (a *Auction) IsEnded() bool {
	return a.Status == StatusEnded
}

// CanBid returns true if a bid can be placed on this auction
func (a *Auction) CanBid() bool {
	//fmt.Println("Checking if auction can bid", a.IsActive(), a.Status)
	return a.Status == StatusActive
}
func (a *Auction) AuctionStarted() bool {
	return a.StartTime.Before(time.Now())
}

// UpdateCurrentPrice updates the current price of the auction
func (a *Auction) UpdateCurrentPrice(newPrice float64) {
	if newPrice > a.CurrentPrice {
		a.CurrentPrice = newPrice
		a.UpdatedAt = time.Now()
	}
}

// EndAuction marks the auction as ended
func (a *Auction) EndAuction() {
	a.Status = StatusEnded
	a.UpdatedAt = time.Now()
}
