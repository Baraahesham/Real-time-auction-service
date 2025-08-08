#!/bin/bash

# Simple database script for Auction Service

case "${1:-}" in
    "start")
        echo "Starting database services (PostgreSQL + Redis)..."
        docker compose up -d postgres redis
        echo "Database services started!"
        ;;
    "stop")
        echo "Stopping database services..."
        docker compose down
        echo "Database services stopped!"
        ;;
    "reset")
        echo "Resetting database services..."
        docker compose down -v
        docker compose up -d postgres redis
        echo "Database services reset!"
        ;;
    *)
        echo "Usage: $0 {start|stop|reset}"
        echo ""
        echo "Commands:"
        echo "  start  - Start database services (PostgreSQL + Redis)"
        echo "  stop   - Stop database services"
        echo "  reset  - Reset database services (delete all data)"
        exit 1
        ;;
esac 