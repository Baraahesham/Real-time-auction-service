package shared

import "errors"

// Domain-specific errors
var (
	// Auction errors
	ErrAuctionNotFound         = errors.New("auction not found")
	ErrAuctionAlreadyEnded     = errors.New("auction already ended")
	ErrAuctionNotAcceptingBids = errors.New("auction is not accepting bids")
	ErrInvalidStartTime        = errors.New("start time cannot be in the past")
	ErrInvalidEndTime          = errors.New("end time must be after start time")
	ErrInvalidStartingPrice    = errors.New("starting price must be greater than 0")
	ErrItemAlreadyInAuction    = errors.New("item is already in an active auction")

	// Bid errors
	ErrBidAmountTooLow        = errors.New("bid amount must be higher than current highest bid")
	ErrBidAmountInvalid       = errors.New("bid amount must be greater than 0")
	ErrBidAmountBelowStarting = errors.New("bid amount must be higher than starting price")
	ErrNoBidsFound            = errors.New("no bids found")
	ErrAuctionNotStarted      = errors.New("auction not started")

	// User errors
	ErrUserNotFound = errors.New("user not found")

	// Item errors
	ErrItemNotFound = errors.New("item not found")

	// Validation errors
	ErrInvalidTimeFormat = errors.New("invalid time format")
	ErrInvalidRequest    = errors.New("invalid request")

	// Database errors
	ErrDatabaseConnection  = errors.New("database connection failed")
	ErrDatabaseQuery       = errors.New("database query failed")
	ErrDatabaseTransaction = errors.New("database transaction failed")

	// WebSocket errors
	ErrWebSocketConnection = errors.New("websocket connection failed")
	ErrWebSocketMessage    = errors.New("websocket message error")

	// WebSocket message validation errors
	ErrMessageTypeRequired   = errors.New("message type is required")
	ErrAuctionIDRequired     = errors.New("auction_id is required")
	ErrInvalidAmount         = errors.New("valid amount is required")
	ErrItemIDRequired        = errors.New("item_id is required")
	ErrStartTimeRequired     = errors.New("start_time is required")
	ErrEndTimeRequired       = errors.New("end_time is required")
	ErrStartingPriceRequired = errors.New("starting_price is required")
	ErrUnknownMessageType    = errors.New("unknown message type")

	// Broadcasting errors
	ErrBroadcastFailed   = errors.New("broadcast failed")
	ErrUserNotSubscribed = errors.New("user not subscribed to auction")

	// WebSocket handler specific errors
	ErrClientEventChannelNotFound = errors.New("client event channel not found")
	ErrInvalidItemIDFormat        = errors.New("invalid item_id format")
)
