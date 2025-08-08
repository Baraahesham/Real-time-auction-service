package config

import (
	"fmt"
	"log"
)

// ExampleUsage demonstrates how to use the configuration system
func ExampleUsage() {
	// Load configuration from .envrc file and environment variables
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate the configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Access configuration values
	//fmt.Printf("Server will run on: %s\n", config.GetServerAddress())
	fmt.Printf("Database URL: %s\n", config.Database.URL)
	fmt.Printf("Log level: %s\n", config.Logging.Level)
	fmt.Printf("WebSocket read buffer size: %d\n", config.WebSocket.ReadBufferSize)

	// Example of using individual config sections
	dbConfig := config.Database
	fmt.Printf("Database connection string: %s\n", dbConfig.GetConnectionString())
}

// ExampleEnvironmentVariables shows how environment variables override .envrc
func ExampleEnvironmentVariables() {
	// You can override .envrc values with environment variables:
	// export PORT=9090
	// export DB_HOST=production-db.example.com
	// export DB_PASSWORD=secure_password

	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Port from environment: %s\n", config.Server.Port)
	fmt.Printf("Database URL from environment: %s\n", config.Database.URL)
}
