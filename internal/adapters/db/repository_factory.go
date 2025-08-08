package db

import (
	"troffee-auction-service/internal/ports/outbound"
)

// RepositoryFactory creates and manages all database repositories
type RepositoryFactory struct {
	conn *Connection
}

// NewRepositoryFactory creates a new repository factory
func NewRepositoryFactory(conn *Connection) *RepositoryFactory {
	return &RepositoryFactory{conn: conn}
}

// GetAuctionRepository returns the auction repository
func (f *RepositoryFactory) GetAuctionRepository() outbound.AuctionRepository {
	return NewAuctionRepository(f.conn)
}

// GetBidRepository returns the bid repository
func (f *RepositoryFactory) GetBidRepository() outbound.BidRepository {
	return NewBidRepository(f.conn)
}

// GetItemRepository returns the item repository
func (f *RepositoryFactory) GetItemRepository() outbound.ItemRepository {
	return NewItemRepository(f.conn)
}

// GetUserRepository returns the user repository
func (f *RepositoryFactory) GetUserRepository() outbound.UserRepository {
	return NewUserRepository(f.conn)
}

// GetAllRepositories returns all repositories in a struct for easy dependency injection
func (f *RepositoryFactory) GetAllRepositories() struct {
	AuctionRepository outbound.AuctionRepository
	BidRepository     outbound.BidRepository
	ItemRepository    outbound.ItemRepository
	UserRepository    outbound.UserRepository
} {
	return struct {
		AuctionRepository outbound.AuctionRepository
		BidRepository     outbound.BidRepository
		ItemRepository    outbound.ItemRepository
		UserRepository    outbound.UserRepository
	}{
		AuctionRepository: f.GetAuctionRepository(),
		BidRepository:     f.GetBidRepository(),
		ItemRepository:    f.GetItemRepository(),
		UserRepository:    f.GetUserRepository(),
	}
}
