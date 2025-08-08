package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Configuration constants
const (
	// Server Configuration
	Port = "PORT"
	Host = "HOST"

	// Database Configuration
	DBURL = "DB_URL"

	// Logging Configuration
	LogLevel  = "LOG_LEVEL"
	LogFormat = "LOG_FORMAT"

	// Redis Configuration
	RedisAddr     = "REDIS_ADDR"
	RedisPassword = "REDIS_PASSWORD"
	RedisDB       = "REDIS_DB"

	// WebSocket Configuration
	WSReadBufferSize  = "WS_READ_BUFFER_SIZE"
	WSWriteBufferSize = "WS_WRITE_BUFFER_SIZE"
	WSMaxWorkers      = 10
	WSMaxCapacity     = 100
)

// Config holds all application configuration
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Logging   LoggingConfig
	WebSocket WebSocketConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port string
	Host string
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string
	Format string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
}

// LoadConfig loads configuration from environment variables and .envrc file
func LoadConfig() (*Config, error) {
	// Set up Viper
	viper.SetConfigName(".envrc")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("../config")

	// Enable environment variable reading
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set default values
	setDefaults()

	// Read config file (optional, will use env vars if file doesn't exist)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found, but that's okay - we'll use environment variables
	}

	config := &Config{
		Server: ServerConfig{
			Port: viper.GetString(Port),
			Host: viper.GetString(Host),
		},
		Database: DatabaseConfig{
			URL: viper.GetString(DBURL),
		},
		Redis: RedisConfig{
			Addr:     viper.GetString(RedisAddr),
			Password: viper.GetString(RedisPassword),
			DB:       viper.GetInt(RedisDB),
		},
		Logging: LoggingConfig{
			Level:  viper.GetString(LogLevel),
			Format: viper.GetString(LogFormat),
		},
		WebSocket: WebSocketConfig{
			ReadBufferSize:  viper.GetInt(WSReadBufferSize),
			WriteBufferSize: viper.GetInt(WSWriteBufferSize),
		},
	}

	return config, nil
}

// setDefaults sets default values for configuration
func setDefaults() {
	// Server defaults
	viper.SetDefault(Port, "8080")
	viper.SetDefault(Host, "localhost")

	// Database defaults
	viper.SetDefault(DBURL, "postgres://postgres:password@localhost:5432/auction_service?sslmode=disable")

	// Redis defaults
	viper.SetDefault(RedisAddr, "localhost:6379")
	viper.SetDefault(RedisPassword, "")
	viper.SetDefault(RedisDB, 0)

	// Logging defaults
	viper.SetDefault(LogLevel, "info")
	viper.SetDefault(LogFormat, "json")

	// WebSocket defaults
	viper.SetDefault(WSReadBufferSize, 1024)
	viper.SetDefault(WSWriteBufferSize, 1024)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}

	if c.Database.URL == "" {
		return fmt.Errorf("database URL is required")
	}

	if c.Redis.Addr == "" {
		return fmt.Errorf("Redis address is required")
	}

	return nil
}
