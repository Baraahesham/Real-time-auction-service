package shared

import "github.com/google/uuid"

// AuctionEndResult represents the result of ending an auction
type AuctionEndResult struct {
	AuctionID  uuid.UUID
	WinnerID   *uuid.UUID
	FinalPrice *float64
	Status     string
}
