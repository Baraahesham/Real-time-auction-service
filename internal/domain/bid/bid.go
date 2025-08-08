package bid

import (
	"time"

	"github.com/google/uuid"
)

// Status represents the status of a bid
type Status string

const (
	StatusAccepted Status = "accepted"
	StatusRejected Status = "rejected"
)

// Bid represents a bid on an auction
type Bid struct {
	ID        uuid.UUID `json:"id"`
	AuctionID uuid.UUID `json:"auction_id"`
	UserID    uuid.UUID `json:"user_id"`
	Amount    float64   `json:"amount"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IsValid returns true if the bid amount is valid (greater than 0)
func (b *Bid) IsValid() bool {
	return b.Amount > 0
}

// Accept marks the bid as accepted
func (b *Bid) Accept() {
	b.Status = StatusAccepted
	b.UpdatedAt = time.Now()
}

// Reject marks the bid as rejected
func (b *Bid) Reject() {
	b.Status = StatusRejected
	b.UpdatedAt = time.Now()
}

// IsAccepted returns true if the bid was accepted
func (b *Bid) IsAccepted() bool {
	return b.Status == StatusAccepted
}

// IsRejected returns true if the bid was rejected
func (b *Bid) IsRejected() bool {
	return b.Status == StatusRejected
}
