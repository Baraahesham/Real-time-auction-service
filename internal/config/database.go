package config

import (
	"github.com/spf13/viper"
)

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	URL string
}

// NewDatabaseConfig creates a new database configuration using Viper
func NewDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		URL: viper.GetString("DB_URL"),
	}
}

// GetConnectionString returns the PostgreSQL connection string
func (c *DatabaseConfig) GetConnectionString() string {
	return c.URL
}
