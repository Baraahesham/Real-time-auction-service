# Real-time-auction-service

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

- Users can create auctions for specific items
- Users can bid on active auctions with real-time updates
- Users can subscribe to multiple auctions simultaneously
- Bids are only accepted during auction active periods
- Bids must exceed the current highest bid amount
- All successful bids are broadcasted to subscribed users
- Only one active auction per item at any time
- Auctions automatically end at their scheduled time

### Non-Functional Requirements

- Bid placement operations prioritize Strong consistency over availability
- System supports multiple service instances
- Service should be scalable

### Out of Scope

- User authentication is assumed to be handled externally, and all incoming requests are considered to be from authenticated users.
## Implementation Details

### 1. Database Choice – Strong Consistency and ACID Guarantees

In a system like this, strong consistency is critical, especially when dealing with financial transactions such as bidding. To ensure data integrity and support ACID properties (Atomicity, Consistency, Isolation, Durability), a relational SQL database is the most suitable choice. It provides transactional guarantees, making it a reliable backbone for operations like bid creation, auction state transitions, and user balances.

### 2. Scalability – Real-Time Auction Updates

Since the auction service is expected to run in multiple instances and integrate with other services (e.g., Payments, Orders), we cannot rely on in-memory state for auction updates. Instead, we use Redis Pub/Sub to propagate real-time events.
- Each auction will have its own Redis channel.
- Interested consumers (e.g., WebSocket gateways or notification services) can subscribe to relevant channels.

This ensures horizontal scalability and decouples message broadcasting from a single service instance.

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

### 3. Bid Consistency – Handling Concurrent Updates

To ensure consistency during bid creation, we will use Optimistic Concurrency Control (OCC).
This means:
- When a user places a new bid, the service will read the current highest bid from the database.
- Before writing the new bid, it checks whether the current highest bid is still the same.
- If another bid was placed in the meantime (i.e., the price changed), the update is aborted or retried.

This avoids race conditions and ensures we don't overwrite a more recent bid.

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

### 4. Handling Auction Expiry – Scheduling Auction End

Auctions must be marked as non-active once they reach their end time.
Two approaches were considered:
- A cron job that periodically queries the database for ended auctions and updates their status. This is will work in this case but not scalable — it causes frequent DB hits.
- Instead, since Redis is already in use, we can leverage Redis Sorted Sets as a lightweight scheduler:
  - Each auction is added to a sorted set with its end timestamp as the score.
  - A background process periodically polls for expired auctions using a time range query.
  - This avoids querying the database repeatedly and enables efficient expiry handling at scale.

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

### 5. WebSocket Performance – Fast Message Handling

To efficiently handle high-throughput WebSocket messages, especially during active auctions, we will use a Worker Pool Pattern.
- Incoming messages are pushed to a buffered queue.
- A pool of workers processes them concurrently.

This model improves throughput, avoids blocking, and ensures low-latency communication with connected clients.


## Architecture

### Hexagonal Architecture (Ports & Adapters)

The system follows **Hexagonal Architecture** principles, providing clear separation of concerns and enabling easy testing and maintenance:

```
┌─────────────────────────────────────────────────────────────┐
│                        Application Layer                     │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐                           │
│  │   Auction   │  │     Bid     │                           │
│  │   Service   │  │   Service   │                           │
│  └─────────────┘  └─────────────┘                           │
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

## Deployment

### Docker Compose Setup

The project includes a `docker-compose.yml` file that sets up PostgreSQL and Redis services for development. The file configures:

- **PostgreSQL**: Database service with persistent volume
- **Redis**: In-memory data store for caching and pub/sub

To use it, run: `docker compose up -d postgres redis`

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
    "amount": 150.00
  },
  "timestamp": 1234567890
}
```

## Future Enhancements

### 1. Testing Infrastructure

The current implementation focuses on core functionality. Future enhancements should include comprehensive testing:

- **Unit Tests**: Add unit tests for all domain logic, application services, and adapters
- **Integration Tests**: Test the complete flow from WebSocket to database operations
- **Performance Tests**: Load testing for WebSocket connections and bid placement
- **End-to-End Tests**: Full auction lifecycle testing with multiple concurrent users

### 2. Distributed Locking for Auction Scheduler

The current auction scheduler processes expired auctions without coordination between multiple service instances. This can lead to race conditions where multiple instances try to end the same auction simultaneously.

**Problem**: When multiple service instances run the scheduler, they might:
- Both detect the same expired auction
- Both attempt to end it simultaneously
- Cause database conflicts or duplicate processing

**Solution**: Implement distributed locking using Redis:
- Use Redis `SET key value NX EX seconds` for atomic lock acquisition
- Each instance attempts to acquire a lock before processing an auction
- Lock key format: `auction:end:{auction_id}`
- Lock duration: 30 seconds (enough time to process auction end)
- If lock acquisition fails, skip that auction (another instance is handling it)

This ensures only one instance processes each auction end, preventing race conditions and improving system reliability.
