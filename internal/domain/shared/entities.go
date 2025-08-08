package shared

import (
	"time"

	"github.com/google/uuid"
)

// User represents an authenticated user in the system
type User struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Item represents an item that can be auctioned
type Item struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
} 