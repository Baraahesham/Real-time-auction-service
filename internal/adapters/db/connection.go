package db

import (
	"database/sql"
	"fmt"

	"troffee-auction-service/internal/config"

	_ "github.com/lib/pq"
)

// Connection represents a database connection
type Connection struct {
	db *sql.DB
}

// NewConnection creates a new database connection
func NewConnection(config *config.Config) (*Connection, error) {
	db, err := sql.Open("postgres", config.Database.GetConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return &Connection{db: db}, nil
}

// GetDB returns the underlying sql.DB instance
func (client *Connection) GetDB() *sql.DB {
	return client.db
}

// Close closes the database connection
func (client *Connection) Close() error {
	return client.db.Close()
}

// BeginTransaction starts a new database transaction
func (client *Connection) BeginTransaction() (*sql.Tx, error) {
	tx, err := client.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

// ExecuteTransaction executes a function within a transaction
func (client *Connection) ExecuteTransaction(fn func(*sql.Tx) error) error {
	tx, err := client.BeginTransaction()
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx failed: %v, rollback failed: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
