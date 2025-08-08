# Troffee Auction Service - System Design Document

A real-time auction service built with Go, featuring WebSocket support, bidding functionality, and real-time updates. This document provides a comprehensive overview of the system architecture, technology choices, and implementation details.

## Table of Contents

1. [System Overview](#system-overview)
2. [Architecture](#architecture)
3. [Technology Stack](#technology-stack)
4. [Data Flow](#data-flow)
5. [Database Design](#database-design)
6. [Scalability & Performance](#scalability--performance)
7. [Consistency Model](#consistency-model)
8. [Deployment](#deployment)

## System Overview

### Functional Requirements

- **Auction Management**: Users can create auctions for specific items
- **Real-time Bidding**: Users can bid on active auctions with real-time updates
- **Subscription System**: Users can subscribe to multiple auctions simultaneously
- **Time-based Rules**: Bids are only accepted during auction active periods
- **Bid Validation**: Bids must exceed the current highest bid amount
- **Real-time Broadcasting**: All successful bids are broadcasted to subscribed users
- **Item Uniqueness**: Only one active auction per item at any time
- **Automatic Expiration**: Auctions automatically end at their scheduled time

### Non-Functional Requirements

- **Strong Consistency**: Bid placement operations prioritize consistency over availability
- **Scalability**: System supports multiple service instances
- **Real-time Performance**: Sub-second latency for WebSocket communications
- **Fault Tolerance**: System continues operating even if some components fail
- **High Availability**: 99.9% uptime target

## Architecture

### Hexagonal Architecture (Ports & Adapters)

The system follows **Hexagonal Architecture** principles, providing clear separation of concerns and enabling easy testing and maintenance:

```
┌─────────────────────────────────────────────────────────────┐
│                        Application Layer                     │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   Auction   │  │     Bid     │  │   User      │          │
│  │   Service   │  │   Service   │  │   Service   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                        Domain Layer                         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   Auction   │  │     Bid     │  │   Shared    │          │
│  │   Domain    │  │   Domain    │  │   Domain    │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                        Ports Layer                          │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   Inbound   │  │   Outbound  │  │   Shared    │          │
│  │   Ports     │  │   Ports     │  │   Ports     │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                      Adapters Layer                         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   WebSocket │  │   Database  │  │    Redis    │          │
│  │   Adapter   │  │   Adapter   │  │   Adapter   │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

### Directory Structure

```
├── cmd/                    # Application entry points
│   └── auction-service/    # Main service entry point
├── internal/              # Internal application code
│   ├── domain/           # Domain entities and business logic
│   │   ├── auction/      # Auction domain logic
│   │   ├── bid/          # Bid domain logic
│   │   └── shared/       # Shared domain types and errors
│   ├── ports/            # Ports (interfaces) for adapters
│   │   ├── inbound/      # Inbound ports (use cases)
│   │   └── outbound/     # Outbound ports (repositories, services)
│   ├── app/              # Application services (use case implementations)
│   ├── adapters/         # Adapters (implementations)
│   │   ├── ws/           # WebSocket adapter
│   │   ├── db/           # Database adapter (PostgreSQL)
│   │   ├── broadcaster/  # Event broadcasting adapter (Redis)
│   │   ├── scheduler/    # Auction scheduler adapter
│   │   └── redis/        # Redis client adapter
│   └── config/           # Configuration management
├── pkg/                  # Shared packages
└── scripts/              # Database scripts and utilities
```

## Technology Stack

### Core Technologies

| Technology | Version | Purpose | Rationale |
|------------|---------|---------|-----------|
| **Go** | 1.21+ | Programming Language | High performance, excellent concurrency support, strong type system |
| **PostgreSQL** | 14+ | Primary Database | ACID compliance, strong consistency, excellent for transactional data |
| **Redis** | 7+ | Caching & Pub/Sub | Fast in-memory operations, built-in pub/sub, sorted sets for scheduling |
| **Gorilla WebSocket** | Latest | WebSocket Library | Mature, well-tested, excellent performance |
| **Viper** | Latest | Configuration | Flexible configuration management with multiple sources |

### Why These Technologies?

#### **Go (Golang)**
- **Concurrency**: Native goroutines and channels for handling thousands of concurrent WebSocket connections
- **Performance**: Near-C performance with garbage collection
- **Simplicity**: Clean syntax and strong standard library
- **WebSocket Support**: Excellent WebSocket libraries and HTTP/2 support

#### **PostgreSQL**
- **ACID Compliance**: Essential for bid placement where strong consistency is required
- **Optimistic Concurrency Control**: Built-in support for OCC patterns
- **JSON Support**: Native JSONB for flexible data storage
- **Performance**: Excellent query optimizer and indexing capabilities

#### **Redis**
- **Pub/Sub**: Native support for real-time event broadcasting
- **Sorted Sets**: Perfect for auction scheduling with end times
- **Performance**: Sub-millisecond latency for real-time operations
- **Scalability**: Horizontal scaling with Redis Cluster

## Data Flow

### WebSocket Connection Flow

```mermaid
sequenceDiagram
    participant Client
    participant WsHandler
    participant AuctionService
    participant BidService
    participant Broadcaster
    participant Redis

    Client->>WsHandler: Connect (user_id)
    WsHandler->>WsHandler: Create Client & Event Channel
    WsHandler->>Broadcaster: Subscribe to Auction Events
    Broadcaster->>Redis: Subscribe to auction:channel
    Redis-->>Broadcaster: Confirmation
    Broadcaster-->>WsHandler: Subscription Confirmed
    WsHandler-->>Client: Connection Established

    Note over Client,Redis: Real-time Event Flow
    loop Event Broadcasting
        AuctionService->>Broadcaster: Publish Event
        Broadcaster->>Redis: Publish to auction:channel
        Redis->>Broadcaster: Event Received
        Broadcaster->>WsHandler: Forward to Event Channel
        WsHandler->>Client: Send WebSocket Message
    end
```

### Bid Placement Flow

```mermaid
sequenceDiagram
    participant Client
    participant WsHandler
    participant BidService
    participant AuctionRepo
    participant BidRepo
    participant Broadcaster
    participant Redis

    Client->>WsHandler: Place Bid Request
    WsHandler->>BidService: PlaceBid()
    
    BidService->>AuctionRepo: Get Auction
    AuctionRepo-->>BidService: Auction Data
    
    BidService->>BidRepo: Get Highest Bid
    BidRepo-->>BidService: Current Highest Bid
    
    BidService->>BidRepo: PlaceBidWithOCC()
    Note over BidRepo: Optimistic Concurrency Control
    BidRepo->>BidRepo: Check Current Price
    BidRepo->>BidRepo: Insert Bid & Update Auction
    BidRepo-->>BidService: Success
    
    BidService->>Broadcaster: Publish Bid Event
    Broadcaster->>Redis: Publish to auction:channel
    Redis->>Broadcaster: Event Confirmed
    Broadcaster-->>BidService: Success
    
    BidService-->>WsHandler: Bid Placed Successfully
    WsHandler-->>Client: Success Response
```

### Auction Expiration Flow

```mermaid
sequenceDiagram
    participant Scheduler
    participant Redis
    participant AuctionService
    participant Broadcaster
    participant WsHandler
    participant Client

    Note over Scheduler: Every Second
    Scheduler->>Redis: ZRANGEBYSCORE expired auctions
    Redis-->>Scheduler: List of Expired Auctions
    
    loop For Each Expired Auction
        Scheduler->>AuctionService: EndAuction()
        AuctionService->>AuctionService: Update Status & Find Winner
        AuctionService-->>Scheduler: Auction End Result
        
        Scheduler->>Broadcaster: Publish Auction Ended Event
        Broadcaster->>Redis: Publish to auction:channel
        Redis->>Broadcaster: Event Confirmed
        
        Broadcaster->>WsHandler: Forward Event
        WsHandler->>Client: Send Auction Ended Message
    end
```

## Database Design

### Schema Overview

```sql
-- Core Tables
CREATE TABLE users (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE items (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE auctions (
    id UUID PRIMARY KEY,
    item_id UUID NOT NULL REFERENCES items(id),
    creator_id UUID NOT NULL REFERENCES users(id),
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    starting_price DECIMAL(10,2) NOT NULL,
    current_price DECIMAL(10,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE bids (
    id UUID PRIMARY KEY,
    auction_id UUID NOT NULL REFERENCES auctions(id),
    user_id UUID NOT NULL REFERENCES users(id),
    amount DECIMAL(10,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE
);
```

### Performance Indexes

```sql
-- Critical Performance Indexes
CREATE INDEX idx_auctions_item_status ON auctions(item_id, status) WHERE status = 'active';
CREATE INDEX idx_auctions_status_created ON auctions(status, created_at DESC);
CREATE INDEX idx_auctions_status_end_time ON auctions(status, end_time) WHERE status = 'active';

CREATE INDEX idx_bids_auction_status_amount ON bids(auction_id, status, amount DESC) WHERE status = 'accepted';
CREATE INDEX idx_bids_auction_created ON bids(auction_id, created_at DESC);
CREATE INDEX idx_bids_user_created ON bids(user_id, created_at DESC);
```

## Scalability & Performance

### Horizontal Scaling Strategy

The system is designed for horizontal scaling with multiple service instances:

#### **Stateless Design**
- No in-memory state shared between instances
- All state stored in PostgreSQL and Redis
- Any instance can handle any request

#### **Distributed Event Broadcasting**
```mermaid
graph TB
    subgraph "Instance 1"
        WS1[WebSocket Handler]
        B1[Broadcaster]
    end
    
    subgraph "Instance 2"
        WS2[WebSocket Handler]
        B2[Broadcaster]
    end
    
    subgraph "Redis Cluster"
        R1[Redis Node 1]
        R2[Redis Node 2]
    end
    
    subgraph "PostgreSQL"
        DB[(Database)]
    end
    
    WS1 --> B1
    WS2 --> B2
    B1 --> R1
    B2 --> R2
    R1 --> WS1
    R2 --> WS2
    B1 --> DB
    B2 --> DB
```

#### **Load Balancing**
- WebSocket connections can be load balanced using sticky sessions
- Database connections use connection pooling
- Redis connections are pooled per instance

### Performance Optimizations

#### **Worker Pool Pattern**
```go
// WebSocket message processing with worker pools
pool := pond.New(
    config.WSMaxWorkers,    // 10 workers
    config.WSMaxCapacity,   // 100 capacity
    pond.Strategy(pond.Balanced()),
)
```

#### **Buffered Channels**
```go
// Event channels with buffering for burst handling
eventChan := make(chan outbound.Event, 100)
sendChan := make(chan *ServerMessage, 100)
```

#### **Database Optimizations**
- **Connection Pooling**: Efficient database connection management
- **Prepared Statements**: Reusable query plans
- **Optimistic Concurrency Control**: Reduced locking overhead
- **Partial Indexes**: Smaller, faster indexes

## Consistency Model

### Strong Consistency for Bids

The system prioritizes **strong consistency** over availability for bid operations:

#### **Optimistic Concurrency Control (OCC)**
```go
// Bid placement with OCC
func (r *BidRepository) PlaceBidWithOCC(ctx context.Context, newBid *bid.Bid, expectedCurrentPrice float64) error {
    return r.conn.ExecuteTransaction(func(tx *sql.Tx) error {
        // 1. Read current auction state
        // 2. Validate expected vs actual price
        // 3. Update only if price hasn't changed
        // 4. Fail if concurrent modification detected
    })
}
```

#### **ACID Properties**
- **Atomicity**: Bid placement and auction update in single transaction
- **Consistency**: Database constraints ensure data integrity
- **Isolation**: OCC prevents race conditions
- **Durability**: PostgreSQL WAL ensures data persistence

### Eventual Consistency for Events

Real-time events use **eventual consistency**:
- Events may be delivered out of order
- Temporary network issues don't block bid placement
- System continues operating during Redis failures

## Deployment

### Docker Compose Setup

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:14
    environment:
      POSTGRES_DB: auction_service
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  auction-service:
    build: .
    ports:
      - "8080:8080"
    environment:
      - DB_URL=postgres://postgres:password@postgres:5432/auction_service
      - REDIS_ADDR=redis:6379
    depends_on:
      - postgres
      - redis
```

### Environment Variables

```bash
# Database Configuration
DB_URL=postgres://user:pass@host:port/db?sslmode=disable

# Redis Configuration
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# Server Configuration
PORT=8080
HOST=localhost

# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# WebSocket Configuration
WS_READ_BUFFER_SIZE=1024
WS_WRITE_BUFFER_SIZE=1024
WS_MAX_WORKERS=10
WS_MAX_CAPACITY=100
```

### Monitoring & Observability

#### **Health Checks**
- `/health` endpoint for load balancer health checks
- Database connection monitoring
- Redis connection monitoring

#### **Logging**
- Structured JSON logging with zerolog
- Request tracing with correlation IDs
- Performance metrics for key operations

#### **Metrics**
- WebSocket connection count
- Bid placement latency
- Auction creation rate
- Event broadcasting throughput

## Getting Started

### Quick Start

```bash
# 1. Clone the repository
git clone <repository-url>
cd troffee-auction-service

# 2. Start dependencies
./scripts/db.sh start

# 3. Run the service
go run cmd/auction-service/main.go

# 4. Connect via WebSocket
# ws://localhost:8080/ws?user_id=550e8400-e29b-41d4-a716-446655440001
```

### Testing

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test ./internal/app -v
```

## API Reference

### WebSocket Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/ws` | WebSocket | Real-time auction updates |

### WebSocket Messages

#### **Client Messages**
```json
{
  "type": "subscribe",
  "auction_id": "uuid",
  "timestamp": 1234567890
}
```

#### **Server Messages**
```json
{
  "type": "bid_placed",
  "auction_id": "uuid",
  "data": {
    "bid_id": "uuid",
    "user_id": "uuid",
    "amount": 150.00
  },
  "timestamp": 1234567890
}
```

## Contributing

1. Follow the hexagonal architecture principles
2. Write tests for new features
3. Update documentation for API changes
4. Follow Go coding standards

## License

[License Information] 